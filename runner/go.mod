module github.com/aspect-build/aspect-gazelle/runner

replace github.com/aspect-build/aspect-gazelle/common => ../common

replace github.com/aspect-build/aspect-gazelle/language/js => ../language/js

replace github.com/aspect-build/aspect-gazelle/language/kotlin => ../language/kotlin

replace github.com/aspect-build/aspect-gazelle/language/orion => ../language/orion

go 1.24.5

require (
	github.com/EngFlow/gazelle_cc v0.4.0 // NOTE: keep in sync with MODULE.bazel
	github.com/aspect-build/aspect-gazelle/common v0.0.0-20251115024249-7cad566bc683
	github.com/aspect-build/aspect-gazelle/language/js v0.0.0-20251115024249-7cad566bc683
	github.com/aspect-build/aspect-gazelle/language/kotlin v0.0.0-20251115024249-7cad566bc683
	github.com/aspect-build/aspect-gazelle/language/orion v0.0.0-20251115024249-7cad566bc683
	github.com/bazel-contrib/rules_python/gazelle v0.0.0-20251012075144-43a5acf8cedf
	github.com/bazelbuild/bazel-gazelle v0.47.0 // NOTE: keep in sync with MODULE.bazel
	github.com/bazelbuild/buildtools v0.0.0-20260119084900-9bdafcfba839
	github.com/bufbuild/rules_buf v0.5.2
	github.com/fatih/color v1.18.0
	github.com/go-git/go-git/v5 v5.16.4
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
	golang.org/x/term v0.39.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/bazel-contrib/rules_jvm v0.31.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.9.2 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/itchyny/gojq v0.12.18-0.20251005142832-e46d0344f209 // indirect
	github.com/itchyny/timefmt-go v0.1.7 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mikefarah/yq/v4 v4.49.1 // indirect
	github.com/msolo/jsonr v0.0.0-20231023064044-62fbfc3a0313 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.starlark.net v0.0.0-20251109183026-be02852a5e1f // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.3 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
