module github.com/aspect-build/aspect-gazelle/language/js

go 1.24.5

replace github.com/aspect-build/aspect-gazelle/common => ../../common

require (
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/bazelbuild/bazel-gazelle v0.46.0-rc2 // NOTE: keep in sync with MODULE.bazel
	github.com/bazelbuild/buildtools v0.0.0-20250930140053-2eb4fccefb52
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/emirpasic/gods v1.18.1
	github.com/msolo/jsonr v0.0.0-20231023064044-62fbfc3a0313 // NOTE: upgrade causes issues with invalid json
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect

require github.com/aspect-build/aspect-gazelle/common v0.0.0-00010101000000-000000000000

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	golang.org/x/sys v0.37.0 // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
)
