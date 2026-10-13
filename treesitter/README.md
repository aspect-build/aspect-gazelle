# aspect-go-treesitter-grammars

Tree-sitter grammar cgo wrappers used by [aspect-gazelle][gazelle] language modules.

This module is intentionally minimal: it bundles each tree-sitter grammar's upstream C source
(via `http_archive`) with a thin cgo `binding.go` wrapper, exposing one `go_library` target per
grammar. Consumers depend only on the grammars they link.

The parent `treesitter` package (AST, query, and filter wrappers) lives in
[aspect-build/aspect-gazelle/common/treesitter][common-tree] and is consumed as a Go module.

## Grammars

| Bazel target | Go import path | Upstream archive |
| --- | --- | --- |
| `@aspect_go_treesitter_grammars//golang` | `.../golang` | `tree-sitter/tree-sitter-go` |
| `@aspect_go_treesitter_grammars//hcl` | `.../hcl` | `tree-sitter-grammars/tree-sitter-hcl` |
| `@aspect_go_treesitter_grammars//java` | `.../java` | `tree-sitter/tree-sitter-java` |
| `@aspect_go_treesitter_grammars//json` | `.../json` | `tree-sitter/tree-sitter-json` |
| `@aspect_go_treesitter_grammars//kotlin` | `.../kotlin` | `fwcd/tree-sitter-kotlin` |
| `@aspect_go_treesitter_grammars//python` | `.../python` | bundled in `smacker/go-tree-sitter` |
| `@aspect_go_treesitter_grammars//ruby` | `.../ruby` | `tree-sitter/tree-sitter-ruby` |
| `@aspect_go_treesitter_grammars//rust` | `.../rust` | `tree-sitter/tree-sitter-rust` |
| `@aspect_go_treesitter_grammars//starlark` | `.../starlark` | `tree-sitter-grammars/tree-sitter-starlark` |
| `@aspect_go_treesitter_grammars//tsx` | `.../tsx` | `tree-sitter/tree-sitter-typescript` |
| `@aspect_go_treesitter_grammars//typescript` | `.../typescript` | `tree-sitter/tree-sitter-typescript` |

`http_archive` blocks are declared in this module's `MODULE.bazel`. They're fetched lazily —
depending on `//typescript` does not download `@tree-sitter-rust`.

## Patches

`patches/go-tree-sitter-abi15.patch` upgrades the bundled C tree-sitter runtime in
[`smacker/go-tree-sitter`][smacker] to ABI 15 so it can parse the grammars above.

The patch is applied automatically when this module is the root (via the dev-only
`go_deps.module_override` in `MODULE.bazel`). Go module overrides are only honored in the
root module, so every consumer must apply it in its own `MODULE.bazel`:

```starlark
bazel_dep(name = "aspect_go_treesitter_grammars", version = "...")

go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.module_override(
    patch_strip = 1,
    patches = ["@aspect_go_treesitter_grammars//patches:go-tree-sitter-abi15.patch"],
    path = "github.com/smacker/go-tree-sitter",
)
```

[gazelle]: https://github.com/aspect-build/aspect-gazelle
[common-tree]: https://github.com/aspect-build/aspect-gazelle/tree/main/common/treesitter
[smacker]: https://github.com/smacker/go-tree-sitter
