package stareval

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bazelbuild/bazel-gazelle/label"
)

// Resolution of `load("@repo//pkg:file.star", ...)` through the Bazel runfiles
// tree, so plugins shipped by an external repository can be loaded.
//
// Plugin paths themselves are workspace-root relative (see host.go), which
// cannot name a file in an external repository. Runfiles can: Bazel stages
// every dependency of the gazelle target under RUNFILES_DIR, keyed by canonical
// repository name, plus a `_repo_mapping` file translating the apparent names
// written in load() to those canonical names.

// The directory the main repository is staged under in a runfiles tree.
const mainRepoRunfilesDir = "_main"

type repoMappingKey struct {
	// Canonical name of the repository containing the file doing the load.
	source string
	// Apparent repository name as written in the load() label.
	apparent string
}

var (
	repoMappingMu    sync.Mutex
	repoMappingCache = map[string]map[repoMappingKey]string{}
)

// runfilesDir returns the root of the runfiles tree, or "" when not running
// under Bazel.
func runfilesDir() string {
	if dir := os.Getenv("RUNFILES_DIR"); dir != "" {
		return dir
	}

	// Fall back to the manifest's directory. The manifest is always written as
	// <runfiles>/MANIFEST, so its parent is the runfiles root.
	if manifest := os.Getenv("RUNFILES_MANIFEST_FILE"); manifest != "" {
		if strings.HasSuffix(manifest, "/MANIFEST") || strings.HasSuffix(manifest, "\\MANIFEST") {
			return filepath.Dir(manifest)
		}
	}

	return ""
}

// ResolveRunfile maps a runfiles short path naming a file in an external
// repository (as produced by $(rootpath), which prefixes those with "../")
// onto an absolute path in the runfiles tree. It reports false when there is no
// runfiles tree or the file is not staged in it.
//
// Main-repository short paths are deliberately not resolved: those are read
// from the source tree so that editing a plugin takes effect without rebuilding
// the gazelle target.
func ResolveRunfile(shortPath string) (string, bool) {
	if !strings.HasPrefix(shortPath, "../") {
		return "", false
	}

	dir := runfilesDir()
	if dir == "" {
		return "", false
	}

	resolved := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(shortPath, "../")))
	if _, err := os.Stat(resolved); err != nil {
		return "", false
	}

	return resolved, true
}

// loadRepoMapping parses the `_repo_mapping` file Bazel writes into the
// runfiles tree. Each line is `source_canonical,apparent_name,target_canonical`
// with an empty source naming the main repository.
//
// A missing file is not an error: WORKSPACE-mode builds have no repo mapping,
// and apparent names are then already canonical.
func loadRepoMapping(dir string) (map[repoMappingKey]string, error) {
	repoMappingMu.Lock()
	defer repoMappingMu.Unlock()

	if cached, ok := repoMappingCache[dir]; ok {
		return cached, nil
	}

	mapping := map[repoMappingKey]string{}

	f, err := os.Open(filepath.Join(dir, "_repo_mapping"))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read runfiles repo mapping: %w", err)
		}
		repoMappingCache[dir] = mapping
		return mapping, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", 3)
		if len(parts) != 3 {
			continue
		}
		mapping[repoMappingKey{source: parts[0], apparent: parts[1]}] = parts[2]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read runfiles repo mapping: %w", err)
	}

	repoMappingCache[dir] = mapping

	return mapping, nil
}

// sourceRepo determines the canonical repository name of the file performing
// the load. Files staged in the runfiles tree are prefixed by their canonical
// repo name; anything else (typically a plugin read from the source tree) is
// attributed to the main repository.
func sourceRepo(dir, fromFile string) string {
	rel, err := filepath.Rel(dir, fromFile)
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

// runfilesPath maps a repository-qualified load() label onto a path in the
// runfiles tree, resolving the apparent repository name relative to the
// repository containing fromFile.
func runfilesPath(fromFile string, l label.Label) (string, error) {
	dir := runfilesDir()
	if dir == "" {
		return "", fmt.Errorf(
			"repository load() requires a Bazel runfiles tree, but RUNFILES_DIR is not set")
	}

	mapping, err := loadRepoMapping(dir)
	if err != nil {
		return "", err
	}

	target, ok := mapping[repoMappingKey{source: sourceRepo(dir, fromFile), apparent: l.Repo}]
	if !ok {
		// Either a WORKSPACE-mode build or a label already naming a canonical
		// repository; both are usable verbatim.
		target = l.Repo
	}
	if target == "" {
		target = mainRepoRunfilesDir
	}

	resolved := filepath.Join(dir, target, filepath.FromSlash(l.Pkg), l.Name)
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf(
			"not found in runfiles at %s (is it in the gazelle target's data?)", resolved)
	}

	return filepath.ToSlash(resolved), nil
}
