package cache

import (
	"sync"

	"github.com/bazelbuild/bazel-gazelle/config"
)

// WatchCache is a disk-backed cache optimized for long-running watch sessions.
// The first access to each path in a session runs the disk-cache hash check
// (catching offline content changes since the last persist); subsequent
// (path, key) hits for that path return the cached value with no file I/O or
// hashing. Any miss also goes through the disk-cache path so entries and
// their content hashes stay in sync on disk. Callers Invalidate paths that
// change mid-session via an external watch signal.
//
// The on-disk format is identical to the disk cache, so entries are
// interchangeable between the two modes across process restarts.
type WatchCache struct {
	*diskCache
	verified sync.Map // path -> struct{} (hash-checked in this session)
}

// NewWatchCache returns an unconfigured WatchCache. Install its NewCache as
// the gazelle cache factory; the persistence file is resolved on first use.
func NewWatchCache() *WatchCache {
	return &WatchCache{
		diskCache: &diskCache{FileComputeCache: NewFileComputeCache()},
	}
}

// NewCache is a CacheFactory. Pass it to SetCacheFactory.
func (c *WatchCache) NewCache(cfg *config.Config) Cache {
	c.initOnce.Do(func() {
		c.file = FilePath(cfg)
		c.read() // diskCache.read — loads entries and content hashes.
	})
	return c
}

// LoadOrStoreFile returns a (path, key) hit with no file I/O once the path
// has been hash-verified in this session. The first access per path (and any
// miss) runs through the disk-cache path so the stored hash stays consistent
// with the stored entry.
func (c *WatchCache) LoadOrStoreFile(root, p, key string, loader FileCompute) (any, bool, error) {
	if _, verified := c.verified.Load(p); verified {
		if e, ok := c.entries.Load(p); ok {
			if v, found := e.(*fileEntry).load(key); found {
				return v, true, nil
			}
		}
	}
	v, cached, err := c.diskCache.LoadOrStoreFile(root, p, key, loader)
	if err == nil {
		c.verified.Store(p, struct{}{})
	}
	return v, cached, err
}

// Invalidate drops the entry, its stored content hash, and the verified mark
// together, so a Persist that follows without re-accessing the path does not
// leave a stale hash on disk and the next access re-runs the hash check.
func (c *WatchCache) Invalidate(paths []string) {
	c.FileComputeCache.Invalidate(paths)
	for _, p := range paths {
		c.contentHashes.Delete(p)
		c.verified.Delete(p)
	}
}

// InvalidateAll wipes every entry, content hash, and verified mark.
func (c *WatchCache) InvalidateAll() {
	c.FileComputeCache.InvalidateAll()
	c.contentHashes.Clear()
	c.verified.Clear()
}
