// Package sa2002 exposes staticcheck's SA2002 — testing.T.Fatal/FailNow
// called from a goroutine, which is not allowed — as a nogo-compatible
// analyzer.
package sa2002

import "github.com/aspect-build/aspect-gazelle/tools/nogo/sa"

var Analyzer = sa.MustFind("SA2002")
