# aspect-treesitter-grammars

Tree-sitter grammar wrappers used by [aspect-gazelle][gazelle] language modules.

This module is intentionally minimal: it exposes one `go_library` target per grammar, each a
thin `binding.go` wrapper that returns the grammar's `*sitter.Language` from
[`odvcencio/gotreesitter`][gotreesitter]. Consumers depend only on the grammars they link.

gotreesitter is a pure-Go tree-sitter runtime with the grammars bundled in, so this module no
longer fetches grammar C sources (`http_archive`) or links a cgo runtime.

The parent `treesitter` package (AST, query, and filter wrappers) lives in
[aspect-build/aspect-gazelle/common/treesitter][common-tree] and is consumed as a Go module.

## Grammars

| Bazel target | Go import path | gotreesitter grammar |
| --- | --- | --- |
| `@aspect_treesitter_grammars//golang` | `.../golang` | `grammars.GoLanguage()` |
| `@aspect_treesitter_grammars//hcl` | `.../hcl` | `grammars.HclLanguage()` |
| `@aspect_treesitter_grammars//java` | `.../java` | `grammars.JavaLanguage()` |
| `@aspect_treesitter_grammars//json` | `.../json` | `grammars.JsonLanguage()` |
| `@aspect_treesitter_grammars//kotlin` | `.../kotlin` | `grammars.KotlinLanguage()` |
| `@aspect_treesitter_grammars//python` | `.../python` | `grammars.PythonLanguage()` |
| `@aspect_treesitter_grammars//ruby` | `.../ruby` | `grammars.RubyLanguage()` |
| `@aspect_treesitter_grammars//rust` | `.../rust` | `grammars.RustLanguage()` |
| `@aspect_treesitter_grammars//starlark` | `.../starlark` | `grammars.StarlarkLanguage()` |
| `@aspect_treesitter_grammars//tsx` | `.../tsx` | `grammars.TsxLanguage()` |
| `@aspect_treesitter_grammars//typescript` | `.../typescript` | `grammars.TypescriptLanguage()` |

[gazelle]: https://github.com/aspect-build/aspect-gazelle
[common-tree]: https://github.com/aspect-build/aspect-gazelle/tree/main/common/treesitter
[gotreesitter]: https://github.com/odvcencio/gotreesitter
