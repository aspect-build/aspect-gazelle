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
| `# gazelle:js_tsconfig enabled\|disabled`               | `enabled`                   |
| Enable generation of `ts_config` rules.<br />This value is inherited by sub-directories and applied relative to each BUILD.<br />The `ts_project(tsconfig)` attribute is *NOT* set and must be done manually if necessary |
| `# gazelle:js_proto enabled\|disabled`                  | 'enabled'                   |
| Enable generation of `ts_proto_library` targets.                                      |
| `# gazelle:js_npm_package enabled\|disabled\|referenced`| `referenced`                |
| Enable generation of `npm_package` targets.<br />DEPRECATED: `referenced` will only generate `npm_package` targets for packages that are referenced by other projects. |
| `# gazelle:js_package_rule_kind js_library\|npm_package`| `npm_package`               |
| The target type to use for the npm package rule. |
| `# gazelle:js_pnpm_lockfile _lockfile_`                 | `pnpm-lock.yaml`            |
| Path to the `pnpm-lock.yaml` file containing available npm packages. <br />This value is inherited by sub-directories and applied relative to each BUILD. |
| `# gazelle:js_tsconfig_ignore _property_`              | `[]`                        |
| Specify a tsconfig related `ts_project` attribute which should not be generated. Attributes include the core `tsconfig` attribute as well as all attributes that must be kept in sync with the tsconfig such as `root_dir`, `declaration`, `incremental`, `composite` etc. Some use cases are (1) when a `ts_project` macro sets the attribute to avoid unnecessary generated code in your BUILD files, (2) when a tsconfig property is unnecessary in the bazel build but can not be removed from the tsconfig.json file. |
| `# gazelle:js_ignore_imports _glob_`                    |                             |
| Imports matching the glob will be ignored when generating BUILD files in the specifying directory and descendants. |
| `# gazelle:js_resolve _glob_ _target_`                  |                             |
| Imports matching the glob will be resolved to the specified target within the specifying directory and descendants.<br />This directive is an extension of the standard `resolve` directive with added glob support and only applying to JavaScript rules. |
| `# gazelle:js_validate_import_statements error\|warn\|off`   | `error`                      |
| Ensure all import statements map to a known dependency. |
| `# gazelle:js_project_naming_convention _name_`         | `{dirname}`                 |
| The format used to generate the name of the main `ts_project` rule. |
| `# gazelle:js_tests_naming_convention _name_`           | `{dirname}_tests`           |
| The format used to generate the name of the test `ts_project` rule. |
| `# gazelle:js_files [custom_target_name] _glob_`        | `**/*.{ts,tsx,mts,cts}`     |
| A glob pattern for files to be included in the main `ts_project` target, or a custom target.<br />Multiple patterns can be specified by using the `js_files` directive multiple times.<br />When specified the inherited configuration is replaced, not extended. |
| `# gazelle:js_test_files [custom_target_name] _glob_`   | `**/*.{spec,test}.{ts,tsx,mts,cts}` |
| Equivalent to `js_files` but for the test `ts_project` target, or a custom test target. |
| `# gazelle:js_npm_package_target_name _name_`           | `{dirname}`                 |
| The format used to generate the name of the `npm_package` target. |
<!-- prettier-ignore-end -->
