package stareval

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

// Resolution of `load("@repo//pkg:file.star", ...)` and of plugin paths naming a
// file in an external repository, through the Bazel runfiles tree.
//
// Plugin paths and load() paths are otherwise workspace-root relative (see
// host.go), which cannot name a file in an external repository. Runfiles can:
// Bazel stages every dependency of the gazelle target there, keyed by canonical
// repository name, alongside the repository mapping that translates the
// apparent names written in load() to those canonical names.

// The directory the main repository is staged under in a runfiles tree.
const mainRepoRunfilesDir = "_main"

var (
	runfilesMu     sync.Mutex
	runfilesLoaded bool
	runfilesEnv    string
	runfilesTree   *runfiles.Runfiles
	runfilesErr    error
)

// bazelRunfiles returns the runfiles tree of the running process, resolving
// both the directory and manifest layouts.
//
// The result is cached against the environment that defines it, which is what
// makes it a cache rather than a one-shot: the tree cannot change underneath a
// fixed environment.
func bazelRunfiles() (*runfiles.Runfiles, error) {
	env := os.Getenv("RUNFILES_DIR") + "\x00" + os.Getenv("RUNFILES_MANIFEST_FILE")

	runfilesMu.Lock()
	defer runfilesMu.Unlock()

	if !runfilesLoaded || env != runfilesEnv {
		runfilesLoaded, runfilesEnv = true, env
		runfilesTree, runfilesErr = runfiles.New()
	}

	return runfilesTree, runfilesErr
}

// ResolveRunfile maps a runfiles short path naming a file in an external
// repository (as produced by $(rootpath), which prefixes those with "../")
// onto a path in the runfiles tree. It reports false when there is no runfiles
// tree or the file is not staged in it.
//
// Main-repository short paths are deliberately not resolved: those are read
// from the source tree so that editing a plugin takes effect without rebuilding
// the gazelle target.
func ResolveRunfile(shortPath string) (string, bool) {
	if !strings.HasPrefix(shortPath, "../") {
		return "", false
	}

	tree, err := bazelRunfiles()
	if err != nil {
		return "", false
	}

	// $(rootpath) prefixes an external repository with exactly one "../".
	// Rlocation rejects the remaining path if it still contains ".." segments,
	// so a path climbing further than that is not a runfiles entry.
	resolved, err := tree.Rlocation(strings.TrimPrefix(shortPath, "../"))
	if err != nil {
		return "", false
	}

	if _, err := os.Stat(resolved); err != nil {
		return "", false
	}

	return filepath.ToSlash(resolved), true
}

// sourceRepo determines the canonical repository holding the file performing
// the load, which decides how the apparent repository names in its load()
// labels are mapped.
//
// Files staged in a runfiles directory are prefixed by their canonical repo
// name. Anything else is attributed to the main repository: a plugin read from
// the source tree, and any file when runfiles are provided as a manifest rather
// than a directory. The latter only matters for a plugin in an external
// repository loading a *third* repository under an apparent name that only it
// knows; loading its own files relatively needs no mapping at all.
func sourceRepo(fromFile string) string {
	dir := os.Getenv("RUNFILES_DIR")
	if dir == "" || fromFile == "" {
		return ""
	}

	rel, err := filepath.Rel(dir, filepath.FromSlash(fromFile))
	if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
		return ""
	}

	repo := rel
	if idx := strings.IndexAny(rel, `/\`); idx >= 0 {
		repo = rel[:idx]
	}
	if repo == mainRepoRunfilesDir {
		return ""
	}

	return repo
}

// runfilesPath maps a file in the repository named by apparentRepo onto a path
// in the runfiles tree, resolving that apparent name relative to the repository
// holding fromFile.
func runfilesPath(fromFile, apparentRepo, repoPath string) (string, error) {
	tree, err := bazelRunfiles()
	if err != nil {
		return "", fmt.Errorf("@%s: a Bazel runfiles tree is required to load from a repository: %w", apparentRepo, err)
	}

	resolved, err := tree.WithSourceRepo(sourceRepo(fromFile)).Rlocation(path.Join(apparentRepo, repoPath))
	if err != nil {
		return "", fmt.Errorf("not found in runfiles: %w", err)
	}

	// A directory-layout runfiles tree resolves any path under it, staged or
	// not, so report the missing input here rather than as a bare open error.
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("not found in runfiles at %s, add it to the gazelle target's data", resolved)
	}

	return filepath.ToSlash(resolved), nil
}
