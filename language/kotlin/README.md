# Kotlin Gazelle Extension

This is a [Gazelle](https://github.com/bazelbuild/bazel-gazelle) `Language` implementation for Kotlin using the [rules_kotlin](https://github.com/bazelbuild/rules_kotlin) `jvm` rules.

> [!NOTE]
> The `aspect_gazelle_kotlin` module is only needed when **building the Gazelle binary from source**. If you use [`aspect_gazelle_prebuilt`](../../prebuilt/), this language is already compiled into the prebuilt binary and no other `aspect_gazelle_*` module is required.

This was originally implemented by @jbedard in https://github.com/aspect-build/aspect-cli
and has been separated out to a standalone git repository and Bazel module.

The work was sponsored by @reddaly and the GoogleX Tapestry team, thanks so much!

## Configuration Directives

This extension supports several custom directives in your `BUILD.bazel` files to control target generation, target naming, and import resolution behavior.

> [!WARNING]
> **Implementation Status**: These directives are currently defined, parsed, and validated into configuration structures within this branch, but **they are not yet wired up to target generation or import resolution in this repository**. 
> - The parser infrastructure to extract the necessary top-level symbols and star imports is implemented in [PR #410](https://github.com/aspect-build/aspect-gazelle/pull/410).
> - The full wiring/implementation of resolution logic utilizing these directives is forthcoming in subsequent PRs (or downstream forks like `gazelle-kotlin`).
> - *TODO: Remove this warning when full resolution/generation support is wired up locally in this extension.*

### `# gazelle:kotlin [enabled|disabled]`
* **Default**: `enabled`
* Controls whether the Kotlin Gazelle extension processes files and generates targets in the current package and its subdirectories.

### `# gazelle:kotlin_generate_mode [package|file|existing]`
* **Default**: `package`
* **Behavior**:
  - `package`: Gazelle runs in automatic target-generation mode. It will automatically construct and insert a `kt_jvm_library` target for directories containing Kotlin source files if they do not already exist.
  - `file`: Auto-generate one library target per Kotlin source file (not yet implemented).
  - `existing`: Gazelle operates in strict mode. It will never generate new `kt_jvm_library` targets. Instead, it expects developers to manually define targets and will only update dependencies for existing library rules. If any Kotlin source files are not listed in the `srcs` of an existing target, Gazelle will raise an error. To skip files, exclude them using `# gazelle:exclude` or `.gitignore`.

### `# gazelle:kotlin_library_suffix [suffix]`
* **Default**: `_lib`
* Configures the target name suffix used for auto-generated `kt_jvm_library` targets. For example, a package directory named `hello` will result in a target named `hello_lib` by default.

### `# gazelle:kotlin_resolve_granularity [package|symbol]`
* **Default**: `package` (to be updated to `symbol`)
* **Behavior**:
  - `package`: Maps and resolves dependencies based on the package statements of source files.
  - `symbol`: Maps and resolves dependencies based on the exact top-level declarations (classes, interfaces, singleton objects, functions, properties, and typealiases) declared within files. This provides precise, declaration-level resolution and is helpful for fine-grained dependency tracking when multiple targets share a package name.
