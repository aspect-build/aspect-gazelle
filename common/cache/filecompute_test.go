package cache

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
)

func fakeConfig(repoName string) *config.Config {
	c := config.New()
	c.RepoName = repoName
	return c
}

// newFileComputeCacheAt returns a FileComputeCache backed by the given path with no initial disk read.
func newFileComputeCacheAt(t *testing.T, file string) *FileComputeCache {
	t.Helper()
	c := NewFileComputeCache()
	c.file = file
	return c
}

// First load of a file invokes the loader and misses the cache.
func TestFileComputeCache_Miss(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := newFileComputeCacheAt(t, filepath.Join(dir, "cache"))
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

// Cached (path, key) hits return the stored value even after the file changes; callers must Invalidate explicitly.
func TestFileComputeCache_HitIgnoresFileChange(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := newFileComputeCacheAt(t, filepath.Join(dir, "cache"))
	compute, calls := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)

	writeTestFile(t, dir, "file.go", "v2")

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Error("expected hit: FileComputeCache does not revalidate on file change")
	}
	if v.(string) != "v1" {
		t.Errorf("expected stale %q, got %q", "v1", v)
	}
	if *calls != 1 {
		t.Errorf("expected no additional compute call, got %d", *calls)
	}
}

// Invalidate removes an entry so the next access re-reads the file and recomputes.
func TestFileComputeCache_Invalidate(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := newFileComputeCacheAt(t, filepath.Join(dir, "cache"))
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

// Entries persist across process restarts via the write/read round trip.
func TestFileComputeCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")
	cacheFile := filepath.Join(dir, "cache")

	c1 := newFileComputeCacheAt(t, cacheFile)
	compute1, _ := makeCompute(t)
	c1.LoadOrStoreFile(dir, "file.go", "key", compute1)
	c1.Persist()

	c2 := newFileComputeCacheAt(t, cacheFile)
	c2.read()

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

// NewCache is idempotent: repeated calls return the same instance without re-reading the persistence file.
func TestFileComputeCache_NewCacheIsSingleton(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "cache")
	t.Setenv("ASPECT_GAZELLE_CACHE", cacheFile)

	c := NewFileComputeCache()
	a := c.NewCache(fakeConfig("repo"))
	b := c.NewCache(fakeConfig("repo"))

	if a != b {
		t.Error("expected NewCache to return the same instance each call")
	}
}

// FilePath returns ASPECT_GAZELLE_CACHE when set, taking precedence over
// the per-repo temp-file fallback.
func TestFilePath_EnvVarOverride(t *testing.T) {
	t.Setenv("ASPECT_GAZELLE_CACHE", "/custom/cache/path")

	got := FilePath(fakeConfig("myrepo"))
	if got != "/custom/cache/path" {
		t.Errorf("expected %q, got %q", "/custom/cache/path", got)
	}
}

// Without the env var, FilePath returns a per-repo file under os.TempDir.
// The RepoName is embedded to keep caches distinct across repos.
func TestFilePath_TempDirFallback(t *testing.T) {
	t.Setenv("ASPECT_GAZELLE_CACHE", "")

	got := FilePath(fakeConfig("myrepo"))
	want := path.Join(os.TempDir(), "aspect-gazelle-myrepo.cache")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// The fallback path reflects the RepoName from the provided config, so two
// repos in the same process do not collide.
func TestFilePath_FallbackUsesRepoName(t *testing.T) {
	t.Setenv("ASPECT_GAZELLE_CACHE", "")

	a := FilePath(fakeConfig("alpha"))
	b := FilePath(fakeConfig("beta"))
	if a == b {
		t.Errorf("expected distinct fallback paths per repo, got %q for both", a)
	}
}
