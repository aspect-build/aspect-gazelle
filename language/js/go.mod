module github.com/aspect-build/aspect-gazelle/language/js

go 1.24.5

replace github.com/aspect-build/aspect-gazelle/common => ../../common

require (
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/aspect-build/aspect-gazelle/common v0.0.0-20251115024249-7cad566bc683
	github.com/bazelbuild/bazel-gazelle v0.47.0 // NOTE: keep in sync with MODULE.bazel
	github.com/bazelbuild/buildtools v0.0.0-20260202105709-e24971d9d1a7
	github.com/emirpasic/gods/v2 v2.0.0-alpha.0.20250312000129-1d83d5ae39fb
	github.com/goexlib/jsonc v0.0.0-20260107034751-fa4908886bd5
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
)
