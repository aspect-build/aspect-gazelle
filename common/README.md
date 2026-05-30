# aspect-gazelle/common

Shared utilities for the Aspect gazelle plugins (`language/*`) and the
`runner`. It provides building blocks such as:

- glob / doublestar matching (`glob.go`) and regex helpers (`regex.go`)
- gazelle directive and error-formatting helpers (`directives.go`, `error.go`)
- set utilities (`set.go`) and a directory walker (`walk.go`)
- BUILD/rule helpers (`rule/`)
- a content-addressed cache (`cache/`)
- structured logging (`logger/`)
- build-info / stamping (`buildinfo/`)
- a tree-sitter parsing wrapper (`treesitter/`)
- Bazel workspace helpers (`bazel/`)

## Consume via go.mod, not bazel_dep

This is published as the Go module
`github.com/aspect-build/aspect-gazelle/common` and is meant to be consumed
**only** through `go.mod` + rules_go's `go_deps` extension — **not** as a Bazel
module via `bazel_dep`.

```starlark
# In the consumer's MODULE.bazel
go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//:go.mod")
use_repo(go_deps, "com_github_aspect_build_aspect_gazelle_common", ...)
```

```go.mod
require github.com/aspect-build/aspect-gazelle/common <version>
```

Its checked-in BUILD files are written specifically for this: root-relative
labels (`//:common`, `//logger`, …) and the canonical `@io_bazel_rules_go` /
`@bazel_gazelle` repo names that `go_deps` exposes, so they resolve as-is in the
generated repo.

Do **not** depend on it via `bazel_dep`. A Bazel module that declares
`go_deps.from_file` on this module's `go.mod` registers (claims) the
`github.com/aspect-build/aspect-gazelle/common` import path as Bazel-provided,
which takes precedence over the version pinned in `go.mod` — that would prevent
downstream modules from selecting a specific `common` version through `go_deps`.
