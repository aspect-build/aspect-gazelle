// The root module's only first-party Go code is //tools/nogo (the shared nogo
// analyzers); common, treesitter, language/* and runner are sibling modules.
// This go.mod backs the go_deps extension for those analyzers and pins the Go
// SDK (@go_sdk), used by the gofmt formatter in //tools/format.
module github.com/aspect-build/aspect-gazelle

go 1.26.4

require (
	golang.org/x/tools v0.45.0
	honnef.co/go/tools v0.7.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
)
