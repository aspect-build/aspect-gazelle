// Package sa6005 exposes staticcheck's SA6005 — inefficient case-insensitive
// string comparison via strings.ToLower/ToUpper equality — as a
// nogo-compatible analyzer.
package sa6005

import "github.com/aspect-build/aspect-gazelle/common/bazel/go/sa"

var Analyzer = sa.MustFind("SA6005")
