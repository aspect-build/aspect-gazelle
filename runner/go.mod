module github.com/aspect-build/aspect-gazelle/runner

replace github.com/aspect-build/aspect-gazelle/common => ../common

replace github.com/aspect-build/aspect-gazelle/language/js => ../language/js

replace github.com/aspect-build/aspect-gazelle/language/kotlin => ../language/kotlin

replace github.com/aspect-build/aspect-gazelle/language/orion => ../language/orion

go 1.24.5

require (
	github.com/EngFlow/gazelle_cc v0.5.0 // NOTE: keep in sync with MODULE.bazel
	github.com/aspect-build/aspect-gazelle/common v0.0.0-20251115024249-7cad566bc683
	github.com/aspect-build/aspect-gazelle/language/js v0.0.0-20251115024249-7cad566bc683
	github.com/aspect-build/aspect-gazelle/language/kotlin v0.0.0-20251115024249-7cad566bc683
	github.com/aspect-build/aspect-gazelle/language/orion v0.0.0-20251115024249-7cad566bc683
	github.com/bazel-contrib/rules_python/gazelle v0.0.0-20260128021939-0057883aa25f
	github.com/bazelbuild/bazel-gazelle v0.47.0 // NOTE: keep in sync with MODULE.bazel
	github.com/bazelbuild/buildtools v0.0.0-20260211083412-859bfffeef82
	github.com/bufbuild/rules_buf v0.5.2
	github.com/fatih/color v1.18.0
	github.com/go-git/go-git/v5 v5.16.5
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0
	golang.org/x/term v0.40.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/bazel-contrib/rules_jvm v0.32.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/emirpasic/gods/v2 v2.0.0-alpha.0.20250312000129-1d83d5ae39fb // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.7.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/goexlib/jsonc v0.0.0-20260107034751-fa4908886bd5 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/hashicorp/hcl/v2 v2.24.0 // indirect
	github.com/itchyny/gojq v0.12.18 // indirect
	github.com/itchyny/timefmt-go v0.1.7 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mikefarah/yq/v4 v4.52.2 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/smacker/go-tree-sitter v0.0.0-20240827094217-dd81d9e9be82 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	github.com/zclconf/go-cty v1.17.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.starlark.net v0.0.0-20260210143700-b62fd896b91b // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.4 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
