package rule

import (
	"fmt"

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

func expandSrcsList(list *bzl.ListExpr) []string {
	srcs := make([]string, 0, len(list.List))
	for _, e := range list.List {
		if str, ok := e.(*bzl.StringExpr); ok {
			srcs = append(srcs, str.Value)
		} else {
			BazelLog.Tracef("skipping non-string src %s", e)
		}
	}
	return srcs
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
