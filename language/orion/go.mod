module github.com/aspect-build/aspect-gazelle/language/orion

go 1.24.5

replace github.com/aspect-build/aspect-gazelle/common => ../../common

require (
	github.com/bazelbuild/bazel-gazelle v0.47.0 // NOTE: keep in sync with MODULE.bazel
	github.com/bazelbuild/buildtools v0.0.0-20251112105957-8e68360eeafa
	github.com/emirpasic/gods v1.18.1
	github.com/itchyny/gojq v0.12.18-0.20251005142832-e46d0344f209
	github.com/mikefarah/yq/v4 v4.48.1
	go.starlark.net v0.0.0-20251109183026-be02852a5e1f
	golang.org/x/sync v0.18.0
)

require github.com/aspect-build/aspect-gazelle/common v0.0.0-20251115024249-7cad566bc683

require (
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/itchyny/timefmt-go v0.1.7 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
)
