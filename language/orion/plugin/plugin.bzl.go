package plugin

import (
	"slices"

	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

// ---------------- TargetSource

var _ rule.BzlExprValue = (*TargetSource)(nil)

func (ts TargetSource) BzlExpr() bzl.Expr {
	return &bzl.StringExpr{Value: ts.Path}
}

// ---------------- Glob

var _ rule.BzlExprValue = (*Glob)(nil)
var _ rule.Merger = (*Glob)(nil)

func (g Glob) BzlExpr() bzl.Expr {
	return rule.GlobValue{Patterns: g.Include, Excludes: g.Exclude}.BzlExpr()
}

// Merge replaces any existing value, which gazelle's default merge cannot do
// for a glob() on either side. An existing glob() with the same patterns is kept
// with its comments and allow_empty. Patterns are compared unordered because
// gazelle sorts the lists inside glob() when writing srcs and deps.
func (g Glob) Merge(other bzl.Expr) bzl.Expr {
	if existing, isGlob := rule.ParseGlobExpr(other); isGlob &&
		sameSet(existing.Patterns, g.Include) &&
		sameSet(existing.Excludes, g.Exclude) {
		return other
	}
	return g.BzlExpr()
}

func sameSet(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}
