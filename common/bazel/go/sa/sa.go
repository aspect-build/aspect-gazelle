// Package sa locates staticcheck checks by name, for the sibling per-check
// packages wired into nogo. nogo requires each dep to export a single
// `var Analyzer *analysis.Analyzer`, while staticcheck publishes its checks
// as a registry slice — each sibling package bridges one check via MustFind.
package sa

import (
	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/staticcheck"
)

func MustFind(name string) *analysis.Analyzer {
	for _, a := range staticcheck.Analyzers {
		if a.Analyzer.Name == name {
			return a.Analyzer
		}
	}
	panic(name + " not found in staticcheck's analyzer registry")
}
