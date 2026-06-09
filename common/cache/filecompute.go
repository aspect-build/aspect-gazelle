package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/config"
)

// fileReadBufPool recycles file-content buffers across reads. Every cache
// implementation reads a file's bytes and hands them to a loader; without
// pooling each read allocates a fresh []byte sized to the file — the dominant
// source of allocation and GC pressure, especially for the noop cache (which
// re-reads on every call) and the disk cache (which reads on every call to
// hash the content). Buffers are reused between reads and grow to the largest
// file seen.
//
// Storing *[]byte (not []byte) avoids boxing the slice header into an interface
// on every Put (staticcheck SA6002).
var fileReadBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 64*1024)
		return &b
	},
}

// maxPooledBuf bounds the capacity we retain. Buffers grown past this (e.g. by a
// large generated file or lockfile) are dropped to GC instead of pinning that
// much memory per pool slot.
const maxPooledBuf = 1 << 20 // 1 MiB

// withFileContent reads name into a pooled buffer, invokes fn with the content,
// and returns the buffer to the pool after fn returns.
//
// fn MUST NOT retain content beyond its return: the buffer is reused for the
// next read. All current callers satisfy this — loaders copy what they keep
// (the JS parser into ParseResult; the orion query path into capture strings,
// Close()-ing its tree before returning), and the disk cache only hashes the
// content. A loader that returned a value aliasing content (an unsafe
// string-over-bytes, or an un-Closed tree-sitter AST) would corrupt it.
func withFileContent(name string, fn func(content []byte) (any, bool, error)) (any, bool, error) {
	bufPtr := fileReadBufPool.Get().(*[]byte)
	defer func() {
		if cap(*bufPtr) <= maxPooledBuf {
			*bufPtr = (*bufPtr)[:0]
			fileReadBufPool.Put(bufPtr)
		}
	}()

	content, err := readFileInto(name, bufPtr)
	if err != nil {
		return nil, false, err
	}
	return fn(content)
}

// readFileInto reads the whole named file into the buffer pointed to by bufPtr,
// growing it (and updating *bufPtr) if the file is larger than its current
// capacity, and returns the populated slice. It mirrors os.ReadFile's
// grow-and-read loop so it tolerates files whose size changes between stat and
// read.
func readFileInto(name string, bufPtr *[]byte) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := (*bufPtr)[:0]

	// Size hint from stat: pre-grow once so the read loop avoids repeated
	// reallocation. +1 (like os.ReadFile) so the final zero-byte read at EOF
	// doesn't force another grow.
	if info, statErr := f.Stat(); statErr == nil {
		if size := info.Size(); size > 0 && int64(int(size)) == size {
			if need := int(size) + 1; cap(buf) < need {
				buf = make([]byte, 0, need)
			}
		}
	}

	for {
		if len(buf) == cap(buf) {
			// Grow: append forces a larger backing array, then reslice to len.
			buf = append(buf, 0)[:len(buf)]
		}
		n, readErr := f.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]
		if readErr != nil {
			*bufPtr = buf // publish any grown buffer so the pool retains it
			if readErr == io.EOF {
				return buf, nil
			}
			return buf, readErr
		}
	}
}

// FilePath returns ASPECT_GAZELLE_CACHE if set, otherwise a per-repo, per-worktree file under os.TempDir.
func FilePath(cfg *config.Config) string {
	if p := os.Getenv("ASPECT_GAZELLE_CACHE"); p != "" {
		return p
	}
	sum := sha256.Sum256([]byte(cfg.RepoRoot))
	return path.Join(os.TempDir(), fmt.Sprintf("aspect-gazelle-%v-%s.cache", cfg.RepoName, hex.EncodeToString(sum[:8])))
}

func init() {
	gob.Register(fileComputeCacheState{})
}

type fileComputeCacheState struct {
	Entries map[string]map[string]any
}

// FileComputeCache is a disk-backed cache whose entries can be directly
// removed by path. Construct one with NewFileComputeCache, set it as the
// active factory with cache.SetCacheFactory(c.NewCache), then call
// Invalidate before each incremental run to evict stale paths.
//
// It implements Cache directly and can be embedded by caches that need to
// augment its behaviour (e.g. symlink resolution or extra metadata).
type FileComputeCache struct {
	entries  *sync.Map
	file     string
	initOnce sync.Once
}

var _ Cache = (*FileComputeCache)(nil)

func NewFileComputeCache() *FileComputeCache {
	return &FileComputeCache{
		entries: &sync.Map{},
	}
}

// Invalidate removes the cache entries for the given workspace-relative paths.
func (c *FileComputeCache) Invalidate(paths []string) {
	for _, p := range paths {
		c.entries.Delete(p)
	}
}

// InvalidateAll drops every cache entry. Use when the host has lost its
// delta state and the in-memory entries can no longer be trusted.
func (c *FileComputeCache) InvalidateAll() {
	c.entries.Clear()
}

// LoadEntries populates the cache from a deserialized map, typically after
// reading a cache file with a custom format.
func (c *FileComputeCache) LoadEntries(m map[string]map[string]any) {
	for p, data := range m {
		c.entries.Store(p, &fileEntry{data: data})
	}
}

// SnapshotEntries returns a serialisable copy of all current entries, for use
// by embedders that write their own cache format.
func (c *FileComputeCache) SnapshotEntries() map[string]map[string]any {
	m := make(map[string]map[string]any)
	c.entries.Range(func(key, value any) bool {
		e := value.(*fileEntry)
		e.mu.RLock()
		row := make(map[string]any, len(e.data))
		for k, v := range e.data {
			row[k] = v
		}
		e.mu.RUnlock()
		m[key.(string)] = row
		return true
	})
	return m
}

// NewCache is a CacheFactory. Pass it to SetCacheFactory.
func (c *FileComputeCache) NewCache(cfg *config.Config) Cache {
	c.initOnce.Do(func() {
		c.file = FilePath(cfg)
		c.read()
	})
	return c
}

func (c *FileComputeCache) read() {
	cacheReader, err := os.Open(c.file)
	if err != nil {
		BazelLog.Tracef("cache: failed to open %q: %v", c.file, err)
		return
	}
	defer cacheReader.Close()

	var v fileComputeCacheState
	dec := gob.NewDecoder(cacheReader)
	if !VerifyCacheVersion(dec, "filecompute", c.file) {
		return
	}
	if e := dec.Decode(&v); e != nil {
		BazelLog.Errorf("cache: failed to read %q: %v", c.file, e)
		return
	}

	c.LoadEntries(v.Entries)
	BazelLog.Infof("cache: loaded %d entries from %q", len(v.Entries), c.file)
}

func (c *FileComputeCache) write() {
	cacheWriter, err := os.OpenFile(c.file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		BazelLog.Errorf("cache: failed to create %q: %v", c.file, err)
		return
	}
	defer cacheWriter.Close()

	m := c.SnapshotEntries()
	enc := gob.NewEncoder(cacheWriter)
	if err := WriteCacheVersion(enc, "filecompute"); err != nil {
		BazelLog.Errorf("cache: failed to write version to %q: %v", c.file, err)
		return
	}
	if e := enc.Encode(fileComputeCacheState{Entries: m}); e != nil {
		BazelLog.Errorf("cache: failed to write %q: %v", c.file, e)
		return
	}
	BazelLog.Debugf("cache: wrote %d entries to %q\n", len(m), c.file)
}

func (c *FileComputeCache) LoadOrStoreFile(root, p, key string, loader FileCompute) (any, bool, error) {
	// Fast path: check the cache before doing any file I/O.
	if e, ok := c.entries.Load(p); ok {
		if v, found := e.(*fileEntry).load(key); found {
			return v, true, nil
		}
	}

	return withFileContent(path.Join(root, p), func(content []byte) (any, bool, error) {
		return c.loadOrStore(p, key, content, loader)
	})
}

// loadOrStore is the inner implementation for callers that have already read
// the file content (e.g. diskCache, which reads it for hash computation).
func (c *FileComputeCache) loadOrStore(p, key string, content []byte, loader FileCompute) (any, bool, error) {
	actual, _ := c.entries.LoadOrStore(p, &fileEntry{data: make(map[string]any)})
	entry := actual.(*fileEntry)

	if v, found := entry.load(key); found {
		return v, true, nil
	}

	v, err := loader(p, content)
	if err == nil {
		v, _ = entry.loadOrStore(key, v)
	}
	return v, false, err
}

func (c *FileComputeCache) Persist() {
	c.write()
}

type fileEntry struct {
	mu   sync.RWMutex
	data map[string]any
}

func (e *fileEntry) load(key string) (any, bool) {
	e.mu.RLock()
	v, ok := e.data[key]
	e.mu.RUnlock()
	return v, ok
}

func (e *fileEntry) loadOrStore(key string, value any) (any, bool) {
	e.mu.Lock()
	if v, ok := e.data[key]; ok {
		e.mu.Unlock()
		return v, true
	}
	e.data[key] = value
	e.mu.Unlock()
	return value, false
}
