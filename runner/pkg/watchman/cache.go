package watchman

import (
	"encoding/gob"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
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
	*cache.FileComputeCache

	w    *WatchmanWatcher
	file string

	symlinks *sync.Map

	lastClockSpec string

	// A reference to the walk cache used in the last gazelle invocation
	// to allow restoration in the next invocation.
	walkCache *sync.Map
}

var _ cache.Cache = (*watchmanCache)(nil)

func NewWatchmanCache(c *config.Config) cache.Cache {
	// Start the watcher
	w := NewWatchman(c.RepoRoot)
	if err := w.Start(); err != nil {
		BazelLog.Fatalf("failed to start the watcher: %v", err)
	}

	return newWatchmanCache(c, w, cache.FilePath(c))
}

// The walk cache of a previous invocation of gazelle.
// Must be a global var that persists across multiple gazelle invocations.
var previousWalkCache *sync.Map

func newWatchmanCache(c *config.Config, w *WatchmanWatcher, diskCachePath string) *watchmanCache {
	wc := &watchmanCache{
		FileComputeCache: cache.NewFileComputeCache(),
		w:                w,
		file:             diskCachePath,
		symlinks:         &sync.Map{},
	}
	wc.populateWalkCache(c)
	wc.read()

	runtime.SetFinalizer(wc, closeWatchmanCache)

	return wc
}

func closeWatchmanCache(c *watchmanCache) {
	c.w.Close()
}

func (c *watchmanCache) populateWalkCache(cfg *config.Config) {
	// If a walk cache was provided also provide the loader to copy the cached entries
	// into any fresh walk cache. This must be invoked from a patched gazelle walk.
	cfg.Exts["aspect:walkCache:load"] = func(m any) {
		cc := 0

		newWalkCache := m.(*sync.Map)
		if c.walkCache != nil {
			c.walkCache.Range(func(key, value any) bool {
				cc++
				newWalkCache.Store(key, value)
				return true
			})
		}

		BazelLog.Debugf("Loaded %d walk cache entries into new walk cache\n", cc)

		// Keep a reference to the walk cache for the new gazelle walk invocation
		// in case of subsequent gazelle invocations.
		previousWalkCache = newWalkCache
	}
}

func invalidateWalkCache(m *sync.Map, staleD string) {
	if staleD == "." || staleD == "" {
		m.Clear()
		return
	}

	m.Range(func(key, value any) bool {
		d := key.(string)
		// Delete the stale directory and any children that may inherit the state
		if staleD == d || len(d) > len(staleD) && strings.HasPrefix(d, staleD) && d[len(staleD)] == '/' {
			m.Delete(key)
		}
		return true
	})
}

func (c *watchmanCache) read() {
	cacheReader, err := os.Open(c.file)
	if err != nil {
		BazelLog.Tracef("Failed to open cache %q: %v", c.file, err)
		return
	}
	defer cacheReader.Close()
	defer func() { previousWalkCache = nil }()

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
		c.lastClockSpec = cs.ClockSpec
		return
	}

	for _, p := range cs.Paths {
		// Discard entries which have changed since the last cache write.
		delete(v.Entries, p)

		// Discard any walk cache entries for the removed/changed path and its parents.
		if previousWalkCache != nil {
			invalidateWalkCache(previousWalkCache, path.Dir(p))
		}
	}

	// Persist the still valid entries as the "old" cache state
	c.FileComputeCache.LoadEntries(v.Entries)
	c.lastClockSpec = cs.ClockSpec
	c.walkCache = previousWalkCache

	// Persist the fact that all persisted paths are not symlinks.
	// Only new paths with no cache entries will require a stat call.
	for k := range v.Entries {
		c.symlinks.LoadOrStore(k, k)
	}

	BazelLog.Infof("Watchman cache: %d/%d entries at clock spec %q", len(v.Entries), loadedEntriesCount, c.lastClockSpec)
}

func (c *watchmanCache) write() {
	cacheWriter, err := os.OpenFile(c.file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		BazelLog.Errorf("Failed to create cache %q: %v", c.file, err)
		return
	}
	defer cacheWriter.Close()

	s := cacheState{
		ClockSpec: c.lastClockSpec,
		Entries:   c.FileComputeCache.SnapshotEntries(),
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

	BazelLog.Debugf("Wrote %d entries at clockspec %q to cache %q\n", len(s.Entries), c.lastClockSpec, c.file)
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
	return c.FileComputeCache.LoadOrStoreFile(root, realP, key, loader)
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
