package plugin

import (
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

// BzlExpr sorts the patterns because gazelle only sorts them inside glob() for
// srcs and deps of new rules, not of rules merged into an existing file.
func (g Glob) BzlExpr() bzl.Expr {
	call := rule.GlobValue{Patterns: g.Include, Excludes: g.Exclude, AllowEmpty: g.AllowEmpty}.BzlExpr()
	bzl.Walk(call, func(e bzl.Expr, _ []bzl.Expr) { bzl.SortStringList(e) })
	return call
}
