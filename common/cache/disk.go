package cache

import (
	"crypto"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
)

/**
 * Cache to disk, keyed by file path. Entries are invalidated when the file's
 * content hash changes. The full cache is discarded on build version mismatch
 * (see VerifyCacheVersion).
 */
func NewDiskCache(cacheFilePath string) Cache {
	c := &diskCache{
		FileComputeCache: NewFileComputeCache(),
		file:             cacheFilePath,
	}
	c.read()
	return c
}

func init() {
	// Register some basic types for gob serialization so languages
	// only have to register custom types.
	gob.Register(map[string]any{})
	gob.Register(map[string]string{})
	gob.Register(map[string][]string{})
	gob.Register(map[string]map[string]any{})
	gob.Register(map[string]map[string]string{})
	gob.Register([]any{})
	gob.Register(diskCacheState{})
}

var _ Cache = (*diskCache)(nil)

type diskCache struct {
	*FileComputeCache

	// Where the cache is persisted to disk.
	file string

	// Maps file path → content hash, used to detect stale entries.
	contentHashes sync.Map
}

type diskCacheState struct {
	Entries       map[string]map[string]any
	ContentHashes map[string]string
}

func computeCacheKey(content []byte) string {
	cacheDigest := crypto.MD5.New()
	cacheDigest.Write(content)
	return hex.EncodeToString(cacheDigest.Sum(nil))
}

func (c *diskCache) read() {
	cacheReader, err := os.Open(c.file)
	if err != nil {
		BazelLog.Infof("Failed to open cache %q: %v", c.file, err)
		return
	}
	defer cacheReader.Close()

	cacheDecoder := gob.NewDecoder(cacheReader)

	if !VerifyCacheVersion(cacheDecoder, "disk", c.file) {
		return
	}

	var v diskCacheState
	if e := cacheDecoder.Decode(&v); e != nil {
		BazelLog.Errorf("Failed to read cache %q: %v", c.file, e)
		return
	}

	c.LoadEntries(v.Entries)
	for p, hash := range v.ContentHashes {
		c.contentHashes.Store(p, hash)
	}

	BazelLog.Infof("Loaded %d entries from cache %q\n", len(v.Entries), c.file)
}

func (c *diskCache) write() {
	cacheWriter, err := os.OpenFile(c.file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		BazelLog.Errorf("Failed to create cache %q: %v", c.file, err)
		return
	}
	defer cacheWriter.Close()

	contentHashes := make(map[string]string)
	c.contentHashes.Range(func(k, v any) bool {
		contentHashes[k.(string)] = v.(string)
		return true
	})

	s := diskCacheState{
		Entries:       c.SnapshotEntries(),
		ContentHashes: contentHashes,
	}

	cacheEncoder := gob.NewEncoder(cacheWriter)

	if err := WriteCacheVersion(cacheEncoder, "disk"); err != nil {
		BazelLog.Errorf("Failed to write cache info to %q: %v", c.file, err)
		return
	}

	if e := cacheEncoder.Encode(s); e != nil {
		BazelLog.Errorf("Failed to write cache %q: %v", c.file, e)
		return
	}

	BazelLog.Infof("Wrote %d entries to cache %q\n", len(s.Entries), c.file)
}

func (c *diskCache) LoadOrStoreFile(root, p, key string, loader FileCompute) (any, bool, error) {
	content, err := os.ReadFile(path.Join(root, p))
	if err != nil {
		return nil, false, err
	}

	contentHash := computeCacheKey(content)

	// Invalidate the cached entry if the file content has changed.
	if existingHash, found := c.contentHashes.Load(p); found {
		if existingHash.(string) != contentHash {
			c.Invalidate([]string{p})
		}
	}
	c.contentHashes.Store(p, contentHash)

	return c.loadOrStore(p, key, content, loader)
}

func (c *diskCache) Persist() {
	c.write()
}
