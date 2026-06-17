package cache

import (
	"encoding/gob"
	"fmt"
	"os"

	"github.com/aspect-build/aspect-gazelle/common/buildinfo"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/config"
)

func init() {
	gob.Register(persistedCacheInfo{})
}

type persistedCacheInfo struct {
	Type    string
	Version string
}

func WriteCacheVersion(encoder *gob.Encoder, cacheType string) error {
	return encoder.Encode(persistedCacheInfo{
		Type:    cacheType,
		Version: buildinfo.GitCommit,
	})
}

// WriteCacheFile truncates+writes a gob-encoded cache file: a version header
// followed by state. Unlike a bare `defer Close()`, it checks the Close error
// so a flush failure surfaced only at close (e.g. NFS commit, ENOSPC under
// delayed allocation) is reported rather than silently leaving a truncated
// cache on disk.
func WriteCacheFile(file, cacheType string, state any) error {
	w, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to create cache %q: %w", file, err)
	}
	// Safety net for the error-return paths below; the happy path closes
	// explicitly so a flush error is not lost.
	defer w.Close()

	encoder := gob.NewEncoder(w)
	if err := WriteCacheVersion(encoder, cacheType); err != nil {
		return fmt.Errorf("failed to write cache info to %q: %w", file, err)
	}
	if err := encoder.Encode(state); err != nil {
		return fmt.Errorf("failed to write cache %q: %w", file, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to flush cache %q: %w", file, err)
	}
	return nil
}

func VerifyCacheVersion(decoder *gob.Decoder, expectedType, file string) bool {
	var pi persistedCacheInfo

	// Read the cache metadata
	if err := decoder.Decode(&pi); err != nil {
		BazelLog.Errorf("Failed to read cache %q: %v", file, err)
		return false
	}

	// Assert the type
	if pi.Type != expectedType {
		BazelLog.Errorf("Cache type mismatch (expected: %q, actual %q), clearing cache %q", expectedType, pi.Type, file)
		return false
	}

	// Assert the version
	if buildinfo.IsStamped() && pi.Version != buildinfo.GitCommit {
		BazelLog.Infof("Cache version mismatch (expected: %q, actual %q), clearing cache %q", buildinfo.GitCommit, pi.Version, file)
		return false
	}

	return true
}

type Cache interface {
	/** Persist any changes to the cache */
	Persist()

	/** Load+Store data computed from file contents.
	 *
	 * If the underlying file has changed since the data was computed, the
	 * loader should return false.
	 *
	 * The file content may or may not be read from disk, depending on the Cache
	 * implementation as well as the cache status.
	 *
	 * The path 'root' is not part of the cache key, but is used to resolve
	 * relative paths in the cache.
	 */
	LoadOrStoreFile(root, path, key string, loader FileCompute) (any, bool, error)
}

type FileCompute = func(path string, content []byte) (any, error)

type CacheFactory = func(c *config.Config) Cache
