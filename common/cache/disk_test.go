package cache

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"testing"
)

// readFile must return identical bytes across sizes (including empty files and
// files larger than a recycled buffer's capacity), and release() must always be
// safe to call. Running several sizes in sequence also exercises buffer reuse
// from the pool, including growing a smaller pooled buffer for a larger file.
func TestReadFile(t *testing.T) {
	dir := t.TempDir()

	sizes := map[string]int{
		"empty": 0,
		"small": 64,
		"large": 64 * 1024,
	}
	for name, size := range sizes {
		t.Run(name, func(t *testing.T) {
			want := make([]byte, size)
			for i := range want {
				want[i] = byte('a' + i%26)
			}
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, want, 0644); err != nil {
				t.Fatal(err)
			}

			got, release, err := readFile(p)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if !bytes.Equal(got, want) {
				t.Errorf("size %d: content mismatch (got %d bytes)", size, len(got))
			}
		})
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func makeCompute(t *testing.T) (func(string, []byte) (any, error), *int) {
	t.Helper()
	n := 0
	return func(_ string, content []byte) (any, error) {
		n++
		return string(content), nil
	}, &n
}

// First load of a file calls the loader.
func TestDiskCache_Miss(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := NewDiskCache(filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("expected miss on first load")
	}
	if v.(string) != "content" {
		t.Errorf("expected %q, got %q", "content", v)
	}
	if *calls != 1 {
		t.Errorf("expected 1 compute call, got %d", *calls)
	}
}

// Second load of an unchanged file returns the cached value without calling the loader.
func TestDiskCache_Hit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := NewDiskCache(filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Error("expected hit on second load of unchanged file")
	}
	if v.(string) != "content" {
		t.Errorf("expected %q, got %q", "content", v)
	}
	if *calls != 1 {
		t.Errorf("expected 1 compute call total, got %d", *calls)
	}
}

// After the file is mutated, the next load is a miss and the loader is called again.
func TestDiskCache_FileChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := NewDiskCache(filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)

	writeTestFile(t, dir, "file.go", "v2")

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("expected miss after file change")
	}
	if v.(string) != "v2" {
		t.Errorf("expected %q, got %q", "v2", v)
	}
	if *calls != 2 {
		t.Errorf("expected 2 compute calls, got %d", *calls)
	}
}

// Different operation keys on the same file are cached independently.
func TestDiskCache_MultipleOpsPerFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := NewDiskCache(filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	_, hit1, _ := c.LoadOrStoreFile(dir, "file.go", "op1", compute)
	_, hit2, _ := c.LoadOrStoreFile(dir, "file.go", "op2", compute)

	if hit1 || hit2 {
		t.Error("expected misses on first load of each op key")
	}
	if *calls != 2 {
		t.Errorf("expected 2 compute calls (one per op), got %d", *calls)
	}

	_, hit3, _ := c.LoadOrStoreFile(dir, "file.go", "op1", compute)
	_, hit4, _ := c.LoadOrStoreFile(dir, "file.go", "op2", compute)

	if !hit3 || !hit4 {
		t.Error("expected hits on second load of each op key")
	}
	if *calls != 2 {
		t.Errorf("expected no additional compute calls, got %d", *calls)
	}
}

// Guards against diskCache re-declaring a `file` field that shadows the
// embedded FileComputeCache.file: with two same-named fields, `c.file`
// resolves to the shallow one and the embedded FileComputeCache's path is
// left permanently empty. There must be exactly one live `file` field, on
// FileComputeCache, holding the configured path.
func TestDiskCache_FilePathOnEmbeddedFileComputeCache(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache")

	c := NewDiskCache(cacheFile).(*diskCache)
	if c.FileComputeCache.file != cacheFile {
		t.Errorf("FileComputeCache.file = %q, want %q (diskCache must not shadow the embedded field)", c.FileComputeCache.file, cacheFile)
	}
}

// A corrupt/unreadable cache file must be ignored (treated as empty) rather than
// crashing; the next load recomputes from scratch.
func TestDiskCache_CorruptFileIgnored(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")
	cacheFile := filepath.Join(dir, "cache")
	if err := os.WriteFile(cacheFile, []byte("not a valid gob stream"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewDiskCache(cacheFile)
	compute, calls := makeCompute(t)

	_, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("expected a miss from a corrupt cache file")
	}
	if *calls != 1 {
		t.Errorf("expected 1 compute call, got %d", *calls)
	}
}

// A cache file whose metadata declares a different cache type must be rejected
// and treated as empty (e.g. a stale file written by another cache).
func TestDiskCache_TypeMismatchIgnored(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")
	cacheFile := filepath.Join(dir, "cache")

	f, err := os.Create(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCacheVersion(gob.NewEncoder(f), "some-other-cache-type"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	c := NewDiskCache(cacheFile)
	compute, calls := makeCompute(t)

	_, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("expected a miss when the cache type does not match")
	}
	if *calls != 1 {
		t.Errorf("expected 1 compute call, got %d", *calls)
	}
}

// Entries written to disk are available after constructing a new cache instance.
func TestDiskCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")
	cacheFile := filepath.Join(dir, "cache")

	c1 := NewDiskCache(cacheFile)
	compute1, _ := makeCompute(t)
	c1.LoadOrStoreFile(dir, "file.go", "key", compute1)
	c1.Persist()

	c2 := NewDiskCache(cacheFile)
	compute2, calls2 := makeCompute(t)
	_, hit, err := c2.LoadOrStoreFile(dir, "file.go", "key", compute2)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Error("expected hit from persisted cache")
	}
	if *calls2 != 0 {
		t.Errorf("expected 0 compute calls after reload, got %d", *calls2)
	}
}
