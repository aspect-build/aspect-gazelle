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
// name; files delivered through a runfiles manifest are reverse-mapped onto
// their manifest key. Anything else is attributed to the main repository, such
// as a plugin read from the source tree.
func sourceRepo(fromFile string) string {
	if fromFile == "" {
		return ""
	}

	if dir := os.Getenv("RUNFILES_DIR"); dir != "" {
		rel, err := filepath.Rel(dir, filepath.FromSlash(fromFile))
		if err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
			repo := rel
			if idx := strings.IndexAny(rel, `/\`); idx >= 0 {
				repo = rel[:idx]
			}
			if repo == mainRepoRunfilesDir {
				return ""
			}
			return repo
		}
	}

	if repo, ok := manifestRepo(fromFile); ok && repo != mainRepoRunfilesDir {
		return repo
	}

	return ""
}

var (
	manifestMu      sync.Mutex
	manifestRevPath string
	manifestRev     map[string]string
)

// manifestRepo determines the canonical repository of a file delivered through
// a runfiles manifest by reverse-mapping its resolved path onto its key.
func manifestRepo(fromFile string) (string, bool) {
	manifest := os.Getenv("RUNFILES_MANIFEST_FILE")
	if manifest == "" {
		return "", false
	}

	manifestMu.Lock()
	defer manifestMu.Unlock()

	if manifestRevPath != manifest {
		manifestRevPath = manifest
		manifestRev = parseManifestRepos(manifest)
	}

	repo, ok := manifestRev[filepath.ToSlash(fromFile)]
	return repo, ok
}

// parseManifestRepos maps each file path in a runfiles manifest back to the
// repository prefix of its key. Each line is "<key> <path>"; a line starting
// with a space escapes both fields (\s space, \b backslash, \n newline).
func parseManifestRepos(manifest string) map[string]string {
	content, err := os.ReadFile(manifest)
	if err != nil {
		return nil
	}

	unescape := strings.NewReplacer(`\s`, " ", `\n`, "\n", `\b`, `\`)

	rev := make(map[string]string)
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSuffix(line, "\r")
		escaped := strings.HasPrefix(line, " ")
		if escaped {
			line = line[1:]
		}

		key, file, ok := strings.Cut(line, " ")
		if !ok || key == "" || file == "" {
			continue
		}
		if escaped {
			key, file = unescape.Replace(key), unescape.Replace(file)
		}

		repo := key
		if idx := strings.IndexByte(key, '/'); idx >= 0 {
			repo = key[:idx]
		}
		rev[filepath.ToSlash(file)] = repo
	}

	return rev
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
