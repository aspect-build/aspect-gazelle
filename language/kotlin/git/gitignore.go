package git

import (
	"bufio"
	"io"
	"os"
	"path"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/config"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// ASPECT_GITIGNORE is the key used to store the gitignore matcher function in the Gazelle config extensions map.
const ASPECT_GITIGNORE = "__aspect:gitignore"

// Directive_GitIgnore is the directive name used in BUILD files to enable or disable gitignore.
const Directive_GitIgnore = "gitignore"

// Internal
const enabledExt = Directive_GitIgnore
const lastConfiguredExt = "gitignore:dir"
const ignorePatternsExt = "gitignore:patterns"

// CollectIgnoreFiles searches for a .gitignore file in the current directory and collects its patterns if found.
func CollectIgnoreFiles(c *config.Config, rel string) {
	// Only parse once per directory.
	if lastCollected, hasCollected := c.Exts[lastConfiguredExt]; hasCollected && lastCollected == rel {
		return
	}
	c.Exts[lastConfiguredExt] = rel

	// Find and add .gitignore files from this directory
	ignoreFilePath := path.Join(c.RepoRoot, rel, ".gitignore")
	ignoreReader, ignoreErr := os.Open(ignoreFilePath)
	if ignoreErr == nil {
		BazelLog.Tracef("Add ignore file %s/.gitignore", rel)
		defer ignoreReader.Close()
		addIgnore(c, rel, ignoreFilePath, ignoreReader)
	} else if !os.IsNotExist(ignoreErr) {
		BazelLog.Errorf("Failed to open %s/.gitignore: %v", rel, ignoreErr)
	}
}

// EnableGitignore registers or unregisters the gitignore matcher function in the config based on the enabled parameter.
func EnableGitignore(c *config.Config, enabled bool) {
	c.Exts[enabledExt] = enabled
	if enabled {
		c.Exts[ASPECT_GITIGNORE] = createMatcherFunc(c)
	} else {
		c.Exts[ASPECT_GITIGNORE] = nil
	}
}

// GetIgnoreFunction retrieves the gitignore matcher function from the config. If gitignore
// is disabled or not set, it returns a function that always returns false.
func GetIgnoreFunction(c *config.Config) func(path string) (ignored bool) {
	configValue := c.Exts[ASPECT_GITIGNORE]
	ignoreNothing := func(_ string) bool { return false }
	if configValue == nil {
		return ignoreNothing
	}
	return configValue.(func(string) bool)
}

func isEnabled(c *config.Config) bool {
	enabled, hasEnabled := c.Exts[enabledExt]
	return hasEnabled && enabled.(bool)
}

func addIgnore(c *config.Config, rel, ignoreFilePath string, ignoreReader io.Reader) {
	var ignorePatterns []gitignore.Pattern

	// Load parent ignore patterns
	if c.Exts[ignorePatternsExt] != nil {
		ignorePatterns = c.Exts[ignorePatternsExt].([]gitignore.Pattern)
	}

	// Append new ignore patterns
	ignorePatterns = append(ignorePatterns, parseIgnore(rel, ignoreFilePath, ignoreReader)...)

	// Persist appended ignore patterns
	c.Exts[ignorePatternsExt] = ignorePatterns

	// Persist a matcher function with the updated ignore patterns if enabled
	if isEnabled(c) {
		c.Exts[ASPECT_GITIGNORE] = createMatcherFunc(c)
	}
}

func parseIgnore(rel, ignoreFilePath string, ignoreReader io.Reader) []gitignore.Pattern {
	var domain []string
	if rel != "" {
		domain = strings.Split(path.Clean(rel), "/")
	}

	matcherPatterns := make([]gitignore.Pattern, 0)

	patternCount := 0
	reader := bufio.NewScanner(ignoreReader)
	for reader.Scan() {
		p := strings.TrimSpace(reader.Text())
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}

		matcherPatterns = append(matcherPatterns, gitignore.ParsePattern(p, domain))
		patternCount++
	}

	BazelLog.Tracef("Parsed %d gitignore patterns from %s", patternCount, ignoreFilePath)

	return matcherPatterns
}

func createMatcherFunc(c *config.Config) func(string) bool {
	patterns, patternsFound := c.Exts[ignorePatternsExt]
	if !patternsFound {
		return nil
	}

	matcher := gitignore.NewMatcher(patterns.([]gitignore.Pattern))
	return func(s string) bool {
		return matcher.Match(strings.Split(s, "/"), false)
	}
}
