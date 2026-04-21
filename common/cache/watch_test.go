package cache

import (
	"os"
	"path/filepath"
	"testing"
)

// newWatchCacheAt returns a WatchCache backed by the given path with no initial disk read.
func newWatchCacheAt(t *testing.T, file string) *WatchCache {
	t.Helper()
	wc := NewWatchCache()
	wc.file = file
	return wc
}

// newWatchCacheLoading returns a WatchCache that reads an existing file.
func newWatchCacheLoading(t *testing.T, file string) *WatchCache {
	t.Helper()
	wc := newWatchCacheAt(t, file)
	wc.read()
	return wc
}

func TestWatchCache_Miss(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := newWatchCacheAt(t, filepath.Join(dir, "cache"))
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

// Cached (path, key) hits return the stored value without file I/O, even when content changed on disk.
func TestWatchCache_HitIgnoresFileChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := newWatchCacheAt(t, filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)

	writeTestFile(t, dir, "file.go", "v2")

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Error("expected hit: watch cache must not revalidate on file change")
	}
	if v.(string) != "v1" {
		t.Errorf("expected stale %q, got %q", "v1", v)
	}
	if *calls != 1 {
		t.Errorf("expected no additional compute call, got %d", *calls)
	}
}

// Different keys on the same path each miss on first access and share a stored entry.
func TestWatchCache_MultipleOpsPerFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := newWatchCacheAt(t, filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	_, hit1, _ := c.LoadOrStoreFile(dir, "file.go", "op1", compute)
	_, hit2, _ := c.LoadOrStoreFile(dir, "file.go", "op2", compute)

	if hit1 || hit2 {
		t.Error("expected misses on first load of each op key")
	}
	if *calls != 2 {
		t.Errorf("expected 2 compute calls, got %d", *calls)
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

func TestWatchCache_Invalidate(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := newWatchCacheAt(t, filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)

	writeTestFile(t, dir, "file.go", "v2")
	c.Invalidate([]string{"file.go"})

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("expected miss after Invalidate")
	}
	if v.(string) != "v2" {
		t.Errorf("expected fresh %q, got %q", "v2", v)
	}
	if *calls != 2 {
		t.Errorf("expected 2 compute calls, got %d", *calls)
	}
}

// Invalidate clears both the cached entry and the content hash so a follow-up Persist doesn't leave a stale hash.
func TestWatchCache_InvalidateClearsContentHash(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := newWatchCacheAt(t, filepath.Join(dir, "cache"))
	compute, _ := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)

	if _, ok := c.contentHashes.Load("file.go"); !ok {
		t.Fatal("expected content hash to be stored after first load")
	}
	if _, ok := c.entries.Load("file.go"); !ok {
		t.Fatal("expected entry to be stored after first load")
	}

	c.Invalidate([]string{"file.go"})

	if _, ok := c.contentHashes.Load("file.go"); ok {
		t.Error("expected content hash to be cleared by Invalidate")
	}
	if _, ok := c.entries.Load("file.go"); ok {
		t.Error("expected entry to be cleared by Invalidate")
	}
}

// After Invalidate + Persist with no re-access, the persisted file omits entry and hash — warm start can't accept a stale hash.
func TestWatchCache_InvalidateBeforePersist(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")
	cacheFile := filepath.Join(dir, "cache")

	c1 := newWatchCacheAt(t, cacheFile)
	compute1, _ := makeCompute(t)
	c1.LoadOrStoreFile(dir, "file.go", "key", compute1)

	c1.Invalidate([]string{"file.go"})
	c1.Persist()

	c2 := newWatchCacheLoading(t, cacheFile)

	if _, ok := c2.contentHashes.Load("file.go"); ok {
		t.Error("expected persisted cache to omit hash for invalidated path")
	}
	if _, ok := c2.entries.Load("file.go"); ok {
		t.Error("expected persisted cache to omit entry for invalidated path")
	}
}

func TestWatchCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")
	cacheFile := filepath.Join(dir, "cache")

	c1 := newWatchCacheAt(t, cacheFile)
	compute1, _ := makeCompute(t)
	c1.LoadOrStoreFile(dir, "file.go", "key", compute1)
	c1.Persist()

	c2 := newWatchCacheLoading(t, cacheFile)
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

func TestWatchCache_WarmStartDetectsOfflineChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")
	cacheFile := filepath.Join(dir, "cache")

	c1 := newWatchCacheAt(t, cacheFile)
	compute1, _ := makeCompute(t)
	c1.LoadOrStoreFile(dir, "file.go", "key", compute1)
	c1.Persist()

	// File changes while no watch session is active (offline change).
	writeTestFile(t, dir, "file.go", "v2")

	c2 := newWatchCacheLoading(t, cacheFile)
	compute2, calls2 := makeCompute(t)
	v, hit, err := c2.LoadOrStoreFile(dir, "file.go", "key", compute2)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("expected miss: warm-start must detect offline content change")
	}
	if v.(string) != "v2" {
		t.Errorf("expected %q, got %q", "v2", v)
	}
	if *calls2 != 1 {
		t.Errorf("expected 1 compute call, got %d", *calls2)
	}
}

func TestWatchCache_DiskFormatInterop(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")
	cacheFile := filepath.Join(dir, "cache")

	d := NewDiskCache(cacheFile)
	compute1, _ := makeCompute(t)
	d.LoadOrStoreFile(dir, "file.go", "key", compute1)
	d.Persist()

	w := newWatchCacheLoading(t, cacheFile)
	compute2, calls2 := makeCompute(t)
	_, hit, err := w.LoadOrStoreFile(dir, "file.go", "key", compute2)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Error("expected hit: watch cache should read a disk-cache file")
	}
	if *calls2 != 0 {
		t.Errorf("expected 0 compute calls after reload, got %d", *calls2)
	}
}

// A new key on an already-verified path re-runs the disk-cache path so the stored hash stays in lockstep with the entry.
func TestWatchCache_KeyMissOnVerifiedPathUpdatesHash(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := newWatchCacheAt(t, filepath.Join(dir, "cache"))
	compute, _ := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "op1", compute)

	h1, ok := c.contentHashes.Load("file.go")
	if !ok {
		t.Fatal("expected hash stored after first load")
	}

	writeTestFile(t, dir, "file.go", "v2")
	c.LoadOrStoreFile(dir, "file.go", "op2", compute)

	h2, _ := c.contentHashes.Load("file.go")
	if h1 == h2 {
		t.Error("expected content hash updated when new content was read for a new key")
	}
}

func TestWatchCache_NewCacheIsSingleton(t *testing.T) {
	dir := t.TempDir()
	cacheFile := filepath.Join(dir, "cache")
	t.Setenv("ASPECT_GAZELLE_CACHE", cacheFile)

	wc := NewWatchCache()
	a := wc.NewCache(fakeConfig("repo"))
	b := wc.NewCache(fakeConfig("repo"))

	if a != b {
		t.Error("expected NewCache to return the same instance each call")
	}

	// A second NewCache call should not re-read the file.
	if err := os.WriteFile(cacheFile, []byte("garbage"), 0644); err != nil {
		t.Fatal(err)
	}

	_ = wc.NewCache(fakeConfig("repo"))
}
