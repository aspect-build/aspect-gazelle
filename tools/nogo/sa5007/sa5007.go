// Package sa5007 exposes staticcheck's SA5007 — infinite recursive call —
// as a nogo-compatible analyzer.
package sa5007

import "github.com/aspect-build/aspect-gazelle/tools/nogo/sa"

var Analyzer = sa.MustFind("SA5007")
