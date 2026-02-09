module github.com/aspect-build/aspect-gazelle/language/kotlin

go 1.24.5

replace github.com/aspect-build/aspect-gazelle/common => ../../common

require (
	github.com/aspect-build/aspect-gazelle/common v0.0.0-00010101000000-000000000000
	github.com/bazel-contrib/rules_jvm v0.32.0
	github.com/bazelbuild/bazel-gazelle v0.47.0
	github.com/emirpasic/gods/v2 v2.0.0-alpha.0.20250312000129-1d83d5ae39fb
	github.com/rs/zerolog v1.34.0
)

require (
	github.com/bazelbuild/buildtools v0.0.0-20260202105709-e24971d9d1a7 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
)
