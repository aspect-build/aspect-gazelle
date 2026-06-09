package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
)

func fakeConfig(repoName string) *config.Config {
	c := config.New()
	c.RepoName = repoName
	return c
}

func fakeConfigAt(repoName, repoRoot string) *config.Config {
	c := fakeConfig(repoName)
	c.RepoRoot = repoRoot
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

// noopCache reads the file and runs the loader, always reporting a miss.
func TestNoopCache_ReadsAndMisses(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "content")

	c := &noopCache{}
	compute, calls := makeCompute(t)

	v, hit, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("noop cache must never report a hit")
	}
	if v.(string) != "content" {
		t.Errorf("expected %q, got %q", "content", v)
	}
	if *calls != 1 {
		t.Errorf("expected 1 compute call, got %d", *calls)
	}
}

// noopCache re-reads on every call: a changed file is reflected immediately and
// the loader runs again (the opposite of FileComputeCache's stale-hit behavior).
func TestNoopCache_AlwaysReReads(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "file.go", "v1")

	c := &noopCache{}
	compute, calls := makeCompute(t)

	c.LoadOrStoreFile(dir, "file.go", "key", compute)
	writeTestFile(t, dir, "file.go", "v2")

	v, _, err := c.LoadOrStoreFile(dir, "file.go", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != "v2" {
		t.Errorf("expected fresh %q, got %q", "v2", v)
	}
	if *calls != 2 {
		t.Errorf("expected 2 compute calls, got %d", *calls)
	}
}

// A file larger than the pool's default buffer capacity exercises the
// read-loop's grow path and must round-trip its content exactly.
func TestNoopCache_LargeFileGrows(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("abcd", 64*1024) // 256 KiB, well past the 64 KiB default
	writeTestFile(t, dir, "big.txt", big)

	c := &noopCache{}
	compute, _ := makeCompute(t)

	v, _, err := c.LoadOrStoreFile(dir, "big.txt", "key", compute)
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != big {
		t.Errorf("large file round-trip mismatch: got %d bytes, want %d", len(v.(string)), len(big))
	}
}

// Reusing one noopCache (and thus the shared buffer pool) across files of
// varying sizes — including large-then-small — must return each file's exact
// content, proving the recycled buffer is correctly resliced and not leaked
// between reads.
func TestNoopCache_PooledBufferReuse(t *testing.T) {
	dir := t.TempDir()
	contents := []string{
		strings.Repeat("X", 200*1024), // forces a grow
		"tiny",                        // reuses the grown buffer; must reslice down
		"",                            // empty file
		strings.Repeat("Y", 100*1024),
		"medium content here",
	}
	for i, want := range contents {
		writeTestFile(t, dir, fmt.Sprintf("f%d", i), want)
	}

	c := &noopCache{}
	compute, _ := makeCompute(t)

	// Two passes to ensure buffers returned to the pool by the first pass are
	// reused by the second without carrying over stale bytes.
	for pass := 0; pass < 2; pass++ {
		for i, want := range contents {
			v, _, err := c.LoadOrStoreFile(dir, fmt.Sprintf("f%d", i), "key", compute)
			if err != nil {
				t.Fatalf("pass %d file %d: %v", pass, i, err)
			}
			if v.(string) != want {
				t.Errorf("pass %d file %d: got %d bytes, want %d", pass, i, len(v.(string)), len(want))
			}
		}
	}
}

// Concurrent reads through the shared pool must each get their own buffer and
// return the correct content (run under -race to catch buffer sharing).
func TestNoopCache_ConcurrentReuse(t *testing.T) {
	dir := t.TempDir()
	const n = 64
	want := make([]string, n)
	for i := 0; i < n; i++ {
		want[i] = strings.Repeat(fmt.Sprintf("%02d", i), (i+1)*512) // distinct, varied sizes
		writeTestFile(t, dir, fmt.Sprintf("f%d", i), want[i])
	}

	c := &noopCache{}
	// A stateless loader: makeCompute's shared counter would itself race,
	// masking what this test checks (that the pooled buffer is not shared).
	compute := func(_ string, content []byte) (any, error) { return string(content), nil }

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for r := 0; r < 20; r++ {
				v, _, err := c.LoadOrStoreFile(dir, fmt.Sprintf("f%d", i), "key", compute)
				if err != nil {
					errs[i] = err
					return
				}
				if v.(string) != want[i] {
					errs[i] = fmt.Errorf("file %d: got %d bytes, want %d", i, len(v.(string)), len(want[i]))
					return
				}
			}
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// A missing file surfaces the open error and does not run the loader.
func TestNoopCache_MissingFile(t *testing.T) {
	c := &noopCache{}
	compute, calls := makeCompute(t)

	_, _, err := c.LoadOrStoreFile(t.TempDir(), "does-not-exist", "key", compute)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if *calls != 0 {
		t.Errorf("expected loader not to run, got %d calls", *calls)
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

// Without the env var, FilePath returns a per-repo, per-worktree file under os.TempDir.
func TestFilePath_TempDirFallback(t *testing.T) {
	t.Setenv("ASPECT_GAZELLE_CACHE", "")

	root := "/tmp/some/repo"
	sum := sha256.Sum256([]byte(root))
	want := path.Join(os.TempDir(), fmt.Sprintf("aspect-gazelle-myrepo-%s.cache", hex.EncodeToString(sum[:8])))

	got := FilePath(fakeConfigAt("myrepo", root))
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// The fallback path reflects the RepoName from the provided config, so two
// repos in the same process do not collide.
func TestFilePath_FallbackUsesRepoName(t *testing.T) {
	t.Setenv("ASPECT_GAZELLE_CACHE", "")

	a := FilePath(fakeConfigAt("alpha", "/tmp/repo"))
	b := FilePath(fakeConfigAt("beta", "/tmp/repo"))
	if a == b {
		t.Errorf("expected distinct fallback paths per repo, got %q for both", a)
	}
}

// Worktrees of the same repo share a RepoName but live at different paths.
func TestFilePath_FallbackDistinguishesWorktrees(t *testing.T) {
	t.Setenv("ASPECT_GAZELLE_CACHE", "")

	a := FilePath(fakeConfigAt("repo", "/tmp/repo-main"))
	b := FilePath(fakeConfigAt("repo", "/tmp/repo-feature"))
	if a == b {
		t.Errorf("expected distinct fallback paths per worktree, got %q for both", a)
	}
	if !strings.Contains(a, "aspect-gazelle-repo-") || !strings.Contains(b, "aspect-gazelle-repo-") {
		t.Errorf("expected RepoName prefix in both paths, got %q and %q", a, b)
	}
}
