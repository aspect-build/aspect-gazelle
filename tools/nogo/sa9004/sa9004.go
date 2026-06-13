// Package sa9004 exposes staticcheck's SA9004 — only the first constant in a
// group has an explicit type — as a nogo-compatible analyzer.
package sa9004

import "github.com/aspect-build/aspect-gazelle/tools/nogo/sa"

var Analyzer = sa.MustFind("SA9004")
