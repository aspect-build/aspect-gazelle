module github.com/aspect-build/aspect-gazelle/language/orion

go 1.24.5

replace github.com/aspect-build/aspect-gazelle/common => ../../common

require (
	github.com/bazelbuild/bazel-gazelle v0.45.1-0.20250924144014-2de7b829fef1 // NOTE: keep in sync with MODULE.bazel
	github.com/bazelbuild/buildtools v0.0.0-20250926132224-6c4b75d79427 // indirect
	github.com/bmatcuk/doublestar/v4 v4.9.1
	github.com/emirpasic/gods v1.18.1
	github.com/fatih/color v1.18.0 // indirect
	github.com/itchyny/gojq v0.12.18-0.20251005142832-e46d0344f209
	github.com/mattn/go-isatty v0.0.20 // indirect
	go.starlark.net v0.0.0-20250717191651-336a4b3a6d1d
	golang.org/x/mod v0.28.0 // indirect
	golang.org/x/sync v0.17.0
)

require (
	github.com/aspect-build/aspect-gazelle/common v0.0.0-00010101000000-000000000000
	github.com/mikefarah/yq/v4 v4.46.1
)

require (
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/itchyny/timefmt-go v0.1.6 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/text v0.29.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
)

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
)
