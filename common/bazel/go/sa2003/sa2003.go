// Package sa2003 exposes staticcheck's SA2003 — deferred Lock right after
// locking, likely meaning to defer Unlock instead — as a nogo-compatible
// analyzer.
package sa2003

import "github.com/aspect-build/aspect-gazelle/common/bazel/go/sa"

var Analyzer = sa.MustFind("SA2003")
