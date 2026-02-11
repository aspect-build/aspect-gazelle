package git

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

type isGitIgnored func(pathParts []string, isDir bool) bool

func processGitignoreFile(rootDir, gitignorePath string, d any) (func(pathParts []string, isDir bool) bool, any) {
	var ignorePatterns []gitignore.Pattern
	if d != nil {
		ignorePatterns = d.([]gitignore.Pattern)
	}

	ignoreReader, ignoreErr := os.Open(path.Join(rootDir, gitignorePath))
	if ignoreErr == nil {
		BazelLog.Tracef("Add gitignore file %s", gitignorePath)
		defer ignoreReader.Close()

		ignorePatterns = append(ignorePatterns, parseIgnore(path.Dir(gitignorePath), ignoreReader)...)
	} else {
		msg := fmt.Sprintf("Failed to open %s: %v", gitignorePath, ignoreErr)
		BazelLog.Error(msg)
		fmt.Printf("%s\n", msg)
	}

	if len(ignorePatterns) == 0 {
		return nil, nil
	}

	// Trim the capacity of the slice to the length to ensure any additional
	// append()ing in the future will reallocate and copy the origina slice.
	ignorePatterns = ignorePatterns[:len(ignorePatterns):len(ignorePatterns)]

	return createMatcherFunc(ignorePatterns), ignorePatterns
}

func createMatcherFunc(ignorePatterns []gitignore.Pattern) isGitIgnored {
	return gitignore.NewMatcher(ignorePatterns).Match
}

func parseIgnore(rel string, ignoreReader io.Reader) []gitignore.Pattern {
	var domain []string
	if rel != "" && rel != "." {
		domain = strings.Split(path.Clean(rel), "/")
	}

	matcherPatterns := make([]gitignore.Pattern, 0)

	reader := bufio.NewScanner(ignoreReader)
	for reader.Scan() {
		p := strings.TrimLeft(reader.Text(), " \t")
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}

		matcherPatterns = append(matcherPatterns, parsePattern(p, domain))
	}

	return matcherPatterns
}

func parsePattern(p string, domain []string) gitignore.Pattern {
	if sp := newSimplePattern(p, domain); sp != nil {
		return sp
	}
	return gitignore.ParsePattern(p, domain)
}

// simplePattern is an optimized gitignore.Pattern for name-only rules (no "/").
// Unlike go-git's simpleNameMatch which calls filepath.Match on every path
// component, this only checks the last component — parent components were
// already tested when those directories were visited during the tree walk.
type simplePattern struct {
	domain    []string
	glob      string
	inclusion bool
	dirOnly   bool
}

// newSimplePattern parses a raw gitignore line into a simplePattern.
// Returns nil if the pattern contains a path separator (not simple),
// otherwise returns a gitignore.Pattern that executes significantly faster
// than go-git's default implementation for simple patterns.
func newSimplePattern(raw string, domain []string) gitignore.Pattern {
	p := &simplePattern{domain: domain}
	s := raw
	if strings.HasPrefix(s, "!") {
		p.inclusion = true
		s = s[1:]
	}
	if !strings.HasSuffix(s, "\\ ") {
		s = strings.TrimRight(s, " ")
	}
	if strings.HasSuffix(s, "/") {
		p.dirOnly = true
		s = s[:len(s)-1]
	}
	if strings.Contains(s, "/") {
		return nil
	}
	p.glob = s
	return p
}

func (p *simplePattern) Match(path []string, isDir bool) gitignore.MatchResult {
	if len(path) <= len(p.domain) {
		return gitignore.NoMatch
	}
	for i, d := range p.domain {
		if path[i] != d {
			return gitignore.NoMatch
		}
	}
	if p.dirOnly && !isDir {
		return gitignore.NoMatch
	}
	name := path[len(path)-1]
	if match, err := filepath.Match(p.glob, name); err != nil || !match {
		return gitignore.NoMatch
	}
	if p.inclusion {
		return gitignore.Include
	}
	return gitignore.Exclude
}
