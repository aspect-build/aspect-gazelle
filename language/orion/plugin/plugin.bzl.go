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
