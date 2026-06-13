// Package sa4006 exposes staticcheck's SA4006 — a value assigned to a
// variable is never read before being overwritten; a forgotten error check
// or dead code — as a nogo-compatible analyzer.
package sa4006

import "github.com/aspect-build/aspect-gazelle/tools/nogo/sa"

var Analyzer = sa.MustFind("SA4006")
