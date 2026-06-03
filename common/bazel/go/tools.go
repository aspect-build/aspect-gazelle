//go:build tools

// Package tools pins Go module dependencies that are not imported by any Go
// source file but are required by Bazel targets, so `go mod tidy` does not
// remove them from go.mod. The "tools" build tag is never set, so this file
// is never compiled.
package tools

import (
	// Keep golang.org/x/tools: the nogo target in BUILD.bazel depends on
	// @org_golang_x_tools//go/analysis/passes/... via the go_deps extension.
	_ "golang.org/x/tools/go/analysis"
)
