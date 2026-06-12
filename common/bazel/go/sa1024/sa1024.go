// Package sa1024 exposes staticcheck's SA1024 — a string cutset contains
// duplicate characters, usually strings.TrimLeft/TrimRight misused as
// TrimPrefix/TrimSuffix — as a nogo-compatible analyzer.
package sa1024

import "github.com/aspect-build/aspect-gazelle/common/bazel/go/sa"

var Analyzer = sa.MustFind("SA1024")
