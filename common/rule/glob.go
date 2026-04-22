package rule

import (
	"fmt"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"github.com/bmatcuk/doublestar/v4"
)

func ExpandSrcs(files []string, expr bzl.Expr) ([]string, error) {
	if list, ok := expr.(*bzl.ListExpr); ok {
		return expandSrcsList(list), nil
	}

	if binary, ok := expr.(*bzl.BinaryExpr); ok && binary.Op == "+" {
		left, err := ExpandSrcs(files, binary.X)
		if err != nil {
			return nil, err
		}
		right, err := ExpandSrcs(files, binary.Y)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil
	}

	g, isGlob := rule.ParseGlobExpr(expr)
	if !isGlob {
		return nil, fmt.Errorf("expected list or glob expression, got %s", expr)
	}

	return expandGlob(files, g), nil
}

// expandSrcsList returns the file-path-shaped entries in list, dropping label references.
func expandSrcsList(list *bzl.ListExpr) []string {
	srcs := make([]string, 0, len(list.List))
	for _, e := range list.List {
		str, ok := e.(*bzl.StringExpr)
		if !ok {
			BazelLog.Tracef("skipping non-string src %s", e)
			continue
		}
		if isLabelRef(str.Value) {
			BazelLog.Tracef("skipping label src %q", str.Value)
			continue
		}
		srcs = append(srcs, str.Value)
	}
	return srcs
}

func isLabelRef(s string) bool {
	if len(s) == 0 {
		return false
	}
	// ":target" or "//pkg:target".
	if s[0] == ':' || strings.HasPrefix(s, "//") {
		return true
	}
	// "@repo//..." — "@" alone isn't enough, directories like "@types" are real paths.
	return s[0] == '@' && strings.Contains(s, "//")
}

func expandGlob(files []string, g rule.GlobValue) []string {
	matches := []string{}

	for _, file := range files {
		matched := false

		for _, pattern := range g.Patterns {
			if doublestar.MatchUnvalidated(pattern, file) {
				matched = true
				break
			}
		}

		if matched {
			for _, pattern := range g.Excludes {
				if doublestar.MatchUnvalidated(pattern, file) {
					matched = false
					break
				}
			}
		}

		if matched {
			matches = append(matches, file)
		}
	}

	return matches
}
