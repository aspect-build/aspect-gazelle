# aspect-treesitter-grammars

Tree-sitter grammar cgo wrappers used by [aspect-gazelle][gazelle] language modules.

This module is intentionally minimal: it bundles each tree-sitter grammar's upstream C source
(via `http_archive`) with a thin cgo `binding.go` wrapper, exposing one `go_library` target per
grammar. Consumers depend only on the grammars they link.

The parent `treesitter` package (AST, query, and filter wrappers) lives in
[aspect-build/aspect-gazelle/common/treesitter][common-tree] and is consumed as a Go module.

## Grammars

| Bazel target | Go import path | Upstream archive |
| --- | --- | --- |
| `@aspect_treesitter_grammars//golang` | `.../golang` | `tree-sitter/tree-sitter-go` |
| `@aspect_treesitter_grammars//hcl` | `.../hcl` | `tree-sitter-grammars/tree-sitter-hcl` |
| `@aspect_treesitter_grammars//java` | `.../java` | `tree-sitter/tree-sitter-java` |
| `@aspect_treesitter_grammars//json` | `.../json` | `tree-sitter/tree-sitter-json` |
| `@aspect_treesitter_grammars//kotlin` | `.../kotlin` | `fwcd/tree-sitter-kotlin` |
| `@aspect_treesitter_grammars//python` | `.../python` | bundled in `smacker/go-tree-sitter` |
| `@aspect_treesitter_grammars//ruby` | `.../ruby` | `tree-sitter/tree-sitter-ruby` |
| `@aspect_treesitter_grammars//rust` | `.../rust` | `tree-sitter/tree-sitter-rust` |
| `@aspect_treesitter_grammars//starlark` | `.../starlark` | `tree-sitter-grammars/tree-sitter-starlark` |
| `@aspect_treesitter_grammars//tsx` | `.../tsx` | `tree-sitter/tree-sitter-typescript` |
| `@aspect_treesitter_grammars//typescript` | `.../typescript` | `tree-sitter/tree-sitter-typescript` |

`http_archive` blocks are declared in this module's `MODULE.bazel`. They're fetched lazily —
depending on `//typescript` does not download `@tree-sitter-rust`.

## Patches

`patches/go-tree-sitter-abi15.patch` upgrades the bundled C tree-sitter runtime in
[`smacker/go-tree-sitter`][smacker] to ABI 15 so it can parse the grammars above.

The patch is applied automatically when this module is the root (via the dev-only
`go_deps.module_override` in `MODULE.bazel`). Downstream consumers that materialize
`github.com/smacker/go-tree-sitter` through their own `go_deps` must reapply it, e.g.:

```python
go_deps_dev = use_extension("@gazelle//:extensions.bzl", "go_deps", dev_dependency = True)
go_deps_dev.module_override(
    patch_strip = 1,
    patches = ["@aspect_treesitter_grammars//patches:go-tree-sitter-abi15.patch"],
    path = "github.com/smacker/go-tree-sitter",
)
```

[gazelle]: https://github.com/aspect-build/aspect-gazelle
[common-tree]: https://github.com/aspect-build/aspect-gazelle/tree/main/common/treesitter
[smacker]: https://github.com/smacker/go-tree-sitter
