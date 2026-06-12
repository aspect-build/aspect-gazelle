// Package sa5011 exposes staticcheck's SA5011 — possible nil pointer
// dereference after a nil check — as a nogo-compatible analyzer.
package sa5011

import "github.com/aspect-build/aspect-gazelle/common/bazel/go/sa"

var Analyzer = sa.MustFind("SA5011")
