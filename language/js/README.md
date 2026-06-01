# JavaScript/TypeScript Gazelle Language

This package is a [Gazelle](https://github.com/bazelbuild/bazel-gazelle) `Language` implementation for [rules_js](https://github.com/aspect-build/rules_js) and [rules_ts](https://github.com/aspect-build/rules_ts).

## Rules

Generated targets include:

- `ts_project` or `js_library` targets for source, tests and custom targets
- `ts_config` targets for `tsconfig.json` files
- `npm_package` or `js_library` targets for npm packages
- `npm_link_all_packages` for linking npm dependencies

By default source targets are generated for tests and library targets. Source globs can be configured using `js_[test_]files glob` directives. Additional custom targets can be generated using the `js_[test_]files target_name glob` directives.

If a `package.json` file exists declaring npm dependencies, a `npm_link_all_packages` target
is generated for declaring depending on individual NPM packages.

If the `package.json` is a pnpm workspace project a `npm_package` or `js_library` target will be generated for the package, the target type may be configured using the `js_package_rule_kind` directive.

Finally, the `import` statements in the source files are parsed, and dependencies are added to the `deps` attribute of the appropriate
`ts_project` target which the source file belongs to. Dependencies may also be found other ways such as from the CommonJS `require` function.

### Directives

<!-- prettier-ignore-start -->
| **Directive**                                           | **Default value**           |
| ------------------------------------------------------- | --------------------------- |
| `# gazelle:js enabled\|disabled`                        | `enabled`                   |
| Enable the JavaScript directives. |
| `# gazelle:js_tsconfig [custom_target_name] enabled\|disabled` | `enabled`              |
| Enable generation of `ts_config` rules and reflection of tsconfig attributes into `ts_project` rules. When a custom target name is provided (e.g. `{dirname}_e2e disabled`), controls generation for that specific target group only. Without a target name, sets the default for all targets.<br />This value is inherited by sub-directories. |
| `# gazelle:js_proto enabled\|disabled\|aspect`          | `enabled`                   |
| `enabled` generates a `ts_proto_library` wrapping each `proto_library`. `aspect` (**EXPERIMENTAL**) skips `ts_proto_library` and resolves proto-generated imports (`*_pb`, `*_pb.js`, `*_pb.d.ts`) directly to the `proto_library` target — requires rules_js >= the aspect-on-proto_library change ([rules_js#a413051](https://github.com/aspect-build/rules_js/commit/a413051)) so `js_library`/`ts_project` can depend on a `proto_library` directly. `disabled` generates no proto-related target and leaves proto-generated imports unresolved. |
| `# gazelle:js_npm_package enabled\|disabled\|referenced`| `referenced`                |
| Enable generation of `npm_package` targets.<br />DEPRECATED: `referenced` will only generate `npm_package` targets for packages that are referenced by other projects. |
| `# gazelle:js_package_rule_kind js_library\|npm_package`| `npm_package`               |
| The target type to use for the npm package rule. |
| `# gazelle:js_visibility [target_name] _labels_...`     |                             |
| Set the `visibility` for generated `ts_project\|js_library` source targets. When the first token is not a label, it is treated as the target name to apply visibility to (defaults to `{dirname}`). |
| `# gazelle:js_pnpm_lockfile _lockfile_`                 | `pnpm-lock.yaml`            |
| Path to the `pnpm-lock.yaml` file containing available npm packages. <br />This value is inherited by sub-directories and applied relative to each BUILD. |
| `# gazelle:js_tsconfig_file [custom_target_name] _filename_`                 | `tsconfig.json`             |
| Path (relative to each package) for locating a `tsconfig.json` file. When a custom target name is provided (e.g. `{dirname}_tests tsconfig.test.json`), sets a per-target override so that target group uses a different tsconfig. Without a target name, sets the default for all targets. This replaces the former `js_test_tsconfig_file` directive. |
| `# gazelle:js_tsconfig_ignore [custom_target_name] _property_`              | `[]`                        |
| Specify a tsconfig related `ts_project` attribute which should not be generated. When a custom target name is provided (e.g. `{dirname}_e2e tsconfig`), the ignore applies only to that target group; otherwise it applies to all targets. Attributes include the core `tsconfig` attribute as well as all attributes that must be kept in sync with the tsconfig such as `root_dir`, `declaration`, `incremental`, `composite` etc. |
| `# gazelle:js_tsconfig_package_deps enabled\|disabled`                      | `disabled`                 |
| Add `package.json` files to the `deps` of generated `ts_config` rules so that `tsc`'s `package.json` lookups (e.g. for `"type"`, `"types"`, `"exports"`) are present in the sandbox. When enabled, a `package.json` co-located with the `tsconfig.json` is added to its `ts_config` rule's `deps`. In addition, every nested Bazel package that contains a `package.json` but no `tsconfig.json` (and is therefore covered by an ancestor's tsconfig) gets a forwarding `ts_config` rule generated whose `src` points to the ancestor's `ts_config` and whose `deps` include the local `package.json`. Applies to all configured tsconfig groups (default and custom). This value is inherited by sub-directories. |
| `# gazelle:js_ignore_imports _glob_`                    |                             |
| Imports matching the glob will be ignored when generating BUILD files in the specifying directory and descendants. |
| `# gazelle:js_assets import\|jsx\|url`                  |                             |
| Specify a comma- or whitespace-separated list of asset types to collect (any of `import`, `jsx`, or `url`). If this directive is not set, all three types are collected by default. For example, `# gazelle:js_assets import` collects only import-based assets, opting out of `jsx` and `url`. |
| `# gazelle:js_resolve _glob_ _target_`                  |                             |
| Imports matching the glob will be resolved to the specified target within the specifying directory and descendants.<br />This directive is an extension of the standard `resolve` directive with added glob support and only applying to JavaScript rules. |
| `# gazelle:js_validate_import_statements error\|warn\|off`   | `error`                      |
| Ensure all import statements map to a known dependency. |
| `# gazelle:js_project_naming_convention _name_`         | `{dirname}`                 |
| The format used to generate the name of the main `ts_project` rule. |
| `# gazelle:js_tests_naming_convention _name_`           | `{dirname}_tests`           |
| The format used to generate the name of the test `ts_project` rule. |
| `# gazelle:js_proto_naming_convention _name_`           | `{proto_library}_ts`        |
| The format used to generate the name of the `ts_proto_library` rule. |
| `# gazelle:js_files [custom_target_name] _glob_`        | `**/*.{ts,tsx,mts,cts}`     |
| A glob pattern for files to be included in the main `ts_project` target, or a custom target.<br />Multiple patterns can be specified by using the `js_files` directive multiple times.<br />When specified the inherited configuration is replaced, not extended. |
| `# gazelle:js_test_files [custom_target_name] _glob_`   | `**/*.{spec,test}.{ts,tsx,mts,cts}` |
| Equivalent to `js_files` but for the test `ts_project` target, or a custom test target. |
| `# gazelle:js_npm_package_target_name _name_`           | `{dirname}`                 |
| The format used to generate the name of the `npm_package` target.<br />The package target depends on the default source group, plus anything it explicitly depends on such as via `package.json` fields (`main`, `exports`, `types`, `typings`) pointing to outputs of other targets. |
<!-- prettier-ignore-end -->

## Build setup

> **The setup below is only required when building the Gazelle binary from source.** Users of [`aspect_gazelle_prebuilt`](../../prebuilt/) can skip it entirely — that module ships a prebuilt Gazelle binary, so you don't compile the Rust parser, or need a Go, Rust, or LLVM toolchain, at all.

The JS/TS import parser is implemented in Rust (using [oxc](https://oxc.rs/)) and linked into the Gazelle binary via cgo. Building this module therefore compiles a Rust static library through [rules_rs](https://github.com/hermeticbuild/rules_rs) and a hermetic LLVM C/C++ toolchain.

`@aspect_gazelle_js` **registers** the Rust and LLVM toolchains itself, so consumers do not register them. But two things cannot propagate through the module graph and must be set by every module that builds this parser — directly, or transitively via a `gazelle_binary` that embeds it.

First, declare `@llvm` so the `@llvm//config:...` flag below resolves (bzlmod only exposes a repo by apparent name to modules that declare it). In `MODULE.bazel`:

```starlark
bazel_dep(name = "llvm", version = "0.8.3")  # match the version @aspect_gazelle_js pins
```

Then add the following to your `.bazelrc`:

```
# Use the hermetic LLVM C/C++ toolchain (provided by @aspect_gazelle_js) to
# build the Rust parser and link it via cgo.
common --repo_env=BAZEL_DO_NOT_DETECT_CPP_TOOLCHAIN=1
common --repo_env=BAZEL_NO_APPLE_CPP_TOOLCHAIN=1
common --linkopt=-no-pie

# Stub libgcc_s; rules_rust tool binaries otherwise link against a non-hermetic
# system libgcc_s that the LLVM sysroot lacks ("unable to find library -lgcc_s").
common --@llvm//config:experimental_stub_libgcc_s=True
```

Notes:

- `--linkopt=-no-pie` works around a Go stdlib + lld PIE-link incompatibility for cgo binaries. It can be dropped once Go reliably links cgo binaries as PIE (expected in Go 1.27 — see [golang/go#77601](https://github.com/golang/go/pull/77601) and [golang/go#76858](https://github.com/golang/go/pull/76858)).
- Within this repository these flags live in [`tools/rust.bazelrc`](../../tools/rust.bazelrc), imported by each workspace that builds the parser (`language/js`, `runner`, `runner/e2e/bin`).
