package watchman

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/aspect-build/aspect-gazelle/common/cache"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/config"
)

func init() {
	gob.Register(cacheState{})
}

type cacheState struct {
	ClockSpec string
	Entries   map[string]map[string]any
}

type watchmanCache struct {
	w *WatchmanWatcher

	file string

	symlinks *sync.Map

	old           map[string]map[string]any
	new           *sync.Map
	lastClockSpec string
}

var _ cache.Cache = (*watchmanCache)(nil)

func NewWatchmanCache(c *config.Config) cache.Cache {
	diskCachePath := os.Getenv("ASPECT_GAZELLE_CACHE")
	if diskCachePath == "" {
		// A default path for the cache file.
		// Try to be unique per repo to allow re-use, while using a temp dir to avoid clutter and indicate
		// the cache is not required.
		diskCachePath = path.Join(os.TempDir(), fmt.Sprintf("aspect-gazelle-%v.cache", c.RepoName))
	}

	// Start the watcher
	w := NewWatchman(c.RepoRoot)
	if err := w.Start(); err != nil {
		log.Fatalf("failed to start the watcher: %v", err)
	}

	return newWatchmanCache(w, diskCachePath)
}

func newWatchmanCache(w *WatchmanWatcher, diskCachePath string) *watchmanCache {
	wc := &watchmanCache{
		w:    w,
		file: diskCachePath,
		old:  map[string]map[string]any{},
		new:  &sync.Map{},

		symlinks: &sync.Map{},
	}
	wc.read()

	runtime.SetFinalizer(wc, closeWatchmanCache)

	return wc
}

func closeWatchmanCache(c *watchmanCache) {
	c.w.Close()
}

func (c *watchmanCache) read() {
	cacheReader, err := os.Open(c.file)
	if err != nil {
		BazelLog.Tracef("Failed to open cache %q: %v", c.file, err)
		return
	}
	defer cacheReader.Close()

	var v cacheState

	cacheDecoder := gob.NewDecoder(cacheReader)

	if !cache.VerifyCacheVersion(cacheDecoder, "watchman", c.file) {
		return
	}

	if e := cacheDecoder.Decode(&v); e != nil {
		BazelLog.Errorf("Failed to read cache %q: %v", c.file, e)
		return
	}

	loadedEntriesCount := len(v.Entries)

	cs, err := c.w.GetDiff(v.ClockSpec)
	if err != nil {
		BazelLog.Errorf("Failed to get diff from watchman: %v", err)
		return
	}

	// If the watcher has restarted, discard the cache.
	if cs.IsFreshInstance {
		BazelLog.Infof("Watchman state is stale, clearing")
		return
	}

	// Discard entries which have changed since the last cache write.
	for _, p := range cs.Paths {
		delete(v.Entries, p)
	}

	// Persist the still valid entries as the "old" cache state
	c.old = v.Entries
	c.lastClockSpec = cs.ClockSpec

	// Persist the fact that all persisted paths are not symlinks.
	// Only new paths with no cache entries will require a stat call.
	for k := range c.old {
		c.symlinks.LoadOrStore(k, k)
	}

	BazelLog.Infof("Watchman cache: %d/%d entries at clock spec %q", len(c.old), loadedEntriesCount, c.lastClockSpec)
}

func (c *watchmanCache) write() {
	cacheWriter, err := os.OpenFile(c.file, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		BazelLog.Errorf("Failed to create cache %q: %v", c.file, err)
		return
	}
	defer cacheWriter.Close()

	m := make(map[string]map[string]any)

	// Convert the sync.Map[sync.Map] to a regular map for serialization.
	c.new.Range(func(key, value interface{}) bool {
		mValue := make(map[string]any)
		value.(*sync.Map).Range(func(k, v interface{}) bool {
			mValue[k.(string)] = v
			return true
		})
		m[key.(string)] = mValue
		return true
	})

	// Include the clock spec and build id in the cache.
	s := cacheState{
		ClockSpec: c.lastClockSpec,
		Entries:   m,
	}

	cacheEncoder := gob.NewEncoder(cacheWriter)

	if err := cache.WriteCacheVersion(cacheEncoder, "watchman"); err != nil {
		BazelLog.Errorf("Failed to write cache info to %q: %v", c.file, err)
		return
	}

	if e := cacheEncoder.Encode(s); e != nil {
		BazelLog.Errorf("Failed to write cache %q: %v", c.file, e)
		return
	}

	BazelLog.Debugf("Wrote %d entries at clockspec %q to cache %q\n", len(m), c.lastClockSpec, c.file)
}

func (c *watchmanCache) Persist() {
	c.write()
}

func (c *watchmanCache) LoadOrStoreFile(root, p, key string, loader cache.FileCompute) (any, bool, error) {
	// Watchman is based on real path locations so symlinks must be resolved to the real path for cache keys.
	realP, err := c.resolveSymlink(root, p)
	if err != nil {
		return nil, false, err
	}

	// Load directly from c.new to potentially convert map[] to sync.Map
	fileMap, hasFileMap := c.new.Load(realP)
	if !hasFileMap {
		// A new map for this file path
		newMap := &sync.Map{}

		// Potentially load the previously persisted map[]
		if oldMap, hasOld := c.old[realP]; hasOld {
			for k, v := range oldMap {
				newMap.Store(k, v)
			}
		}

		fileMap, _ = c.new.LoadOrStore(realP, newMap)
	}

	// Load any cached result from the file specific sync.Map
	v, found := fileMap.(*sync.Map).Load(key)
	if found {
		return v, true, nil
	}

	// Uncached and must be computed from file content
	content, err := os.ReadFile(path.Join(root, realP))
	if err != nil {
		return nil, false, err
	}

	// Compute the value and store it in the file specific sync.Map
	v, err = loader(p, content)
	if err == nil {
		v, found = fileMap.(*sync.Map).LoadOrStore(key, v)
	}

	return v, found, err
}

func (c *watchmanCache) resolveSymlink(root, p string) (string, error) {
	realP, isLinkKnown := c.symlinks.Load(p)
	if !isLinkKnown {
		// Not a link by default
		realP = p

		// Check if the path is a symlink using Lstat (doesn't follow symlinks)
		fi, err := os.Lstat(path.Join(root, p))
		if err != nil {
			return p, err
		}

		// Resolve symlinks relative to the root dir
		if fi.Mode()&os.ModeSymlink != 0 {
			if evalPath, err := filepath.EvalSymlinks(path.Join(root, p)); err == nil {
				if relPath, err := filepath.Rel(root, evalPath); err == nil {
					realP = relPath
				}
			}
		}

		// Store the resolved path (or the original path if not a link)
		realP, _ = c.symlinks.LoadOrStore(p, realP)

		// Store the realpath map to itself to avoid a lstat on that in the future
		if p != realP {
			realP, _ = c.symlinks.LoadOrStore(realP, realP)
		}
	}

	return realP.(string), nil
}
