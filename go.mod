// The root module has no first-party Go code (common, treesitter, language/*
// and runner are sibling modules). This go.mod exists only so go_sdk.from_file
// can pin the Go SDK (@go_sdk), used by the gofmt formatter in //tools/format.
module github.com/aspect-build/aspect-gazelle

go 1.26.4
