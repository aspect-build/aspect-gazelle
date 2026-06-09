# Aspect Gazelle - BUILD generation

> [!NOTE]
> This repository uses the [Aspect CLI](https://github.com/aspect-build/aspect-cli) for CI and local development.
> See the [docs](https://docs.aspect.build/cli/overview) and [install instructions](https://docs.aspect.build/cli/install) to get started.

## Gazelle Languages

### JavaScript

See [language/js](./language/js).

### Aspect Extensions

The [language/orion](./language/orion) package provides a Gazelle `Language` implementation enabling Aspect Extension Language (AXL, a Starlark dialect) for writing extensions

## Gazelle Enhancements

We provide a variety of enhancements to Gazelle.

The [runner](./runner) enables these enhancements automatically, otherwise manual setup (including patching Gazelle) is required.

The runner has Aspect-endorsed Gazelle languages built in; further Gazelle languages (written with the Go SDK) cannot be added without changes to the [runner](./runner). The built-in languages — the names accepted by the `languages` attribute — are [`buf`](https://github.com/bufbuild/rules_buf/tree/main/gazelle/buf), [`cc`](https://github.com/EngFlow/gazelle_cc/tree/main/language/cc), [`go`](https://github.com/bazel-contrib/bazel-gazelle/tree/master/language/go), [`js`](./language/js), [`kotlin`](./language/kotlin), [`orion`](./language/orion) (AXL extensions), [`proto`](https://github.com/bazel-contrib/bazel-gazelle/tree/master/language/proto), [`python`](https://github.com/bazel-contrib/rules_python/tree/main/gazelle), [`starlark`](https://github.com/bazelbuild/bazel-skylib/tree/main/gazelle/bzl) and [`visibility_extension`](https://github.com/bazel-contrib/bazel-gazelle/tree/master/language/bazel/visibility). This same fixed language set is what the [prebuilt](#prebuilt) binary ships. More may be added upon request (file an issue) if the quality is good and maintenance is likely.

### Gitignore

Support for `.gitignore` when generating BUILD files. Files (and directories) matched by any `.gitignore` in the workspace are skipped during the walk, so they don't become BUILD-file sources.

Enabled by default. To opt out, pass `--gitignore=false` to gazelle (in `extra_args` on the gazelle target, or on the command line):

```sh
bazel run //:gazelle -- --gitignore=false
```

### Caching

File based caching of any file analysis by Gazelle language implementations.

Basic caching can be enabled by setting the `ASPECT_CONFIGURE_CACHE` environment variable to a path (e.g. `~/.cache/aspect-gazelle.cache`) for loading+persisting the cache between Gazelle runs.

Further functionality includes [watchman](https://facebook.github.io/watchman/) and other utilities for Gazelle language implementations.

See the [common/cache](./common/cache)

### `--watch` mode

The [runner](./runner) supports a `--watch` mode that uses [watchman](https://facebook.github.io/watchman/) to monitor the filesystem for changes and regenerate BUILD files as needed. This automatically enables the watchman based caching provided by the [common/cache](./common/cache) package.

## Prebuilt

The [prebuilt](./prebuilt) module provides a prebuilt version of the [runner](./runner).

This is a drop-in replacement for the Gazelle binary, with all the runner enhancements and extensions included, and without the need for any compilation of Gazelle or Gazelle languages on the user's machine. This avoids pulling in transitive bzlmod dependencies such as rules_go, rules_python, and rules_rs (with its LLVM toolchain), which are otherwise required to build Gazelle from source.

> [!NOTE]
> `aspect_gazelle_prebuilt` is the only module you need — every language and extension is already compiled into the prebuilt binary. The other `aspect_gazelle_*` modules (`aspect_gazelle_js`, `aspect_gazelle_kotlin`, `aspect_gazelle_orion`, `aspect_gazelle_runner`, ...) are only used when building the Gazelle binary from source; do not add them to your `MODULE.bazel` when using the prebuilt module.

### Why use a prebuilt binary?

Gazelle is commonly built from source on developer's machines, using a Go toolchain.
However this doesn't always work well.

Here's a representative take:

https://plaid.com/blog/hello-bazel/

> A week later, reports started coming in from users complaining that running the tool was taking too long, sometimes multiple minutes. This took us by surprise – the team had not encountered any slowness in the 6 months leading up to that moment, and the generation was only taking a handful of seconds in CI. Once we added instrumentation to our tooling, we were surprised to find a median duration of about 20 seconds and a p95 duration extending to several minutes.

Not only can it be slow, it can often be broken. That's because Gazelle extensions don't have to be written in pure Go.

For example see this issue, where the Python extension depends on a C library called TreeSitter, which forces projects to setup a functional and hermetic cc toolchain:

https://github.com/bazel-contrib/rules_python/issues/1913

### Install & Setup

Add to your `MODULE.bazel`:

```starlark
bazel_dep(name = "aspect_gazelle_prebuilt", version = "...")
```

Add a `gazelle` target to your `BUILD` file:

```starlark
load("@aspect_gazelle_prebuilt//:def.bzl", "aspect_gazelle")

aspect_gazelle(
    name = "gazelle",
    languages = ["js", "python", ...],
    extensions = ["//tools/gazelle:my_rule.star", ...],
    # Also create a `gazelle.check` target that fails if BUILD files are stale.
    with_check = True,
)
```

Continue as normal from the [gazelle Usage](https://github.com/bazel-contrib/bazel-gazelle#usage) docs.
See [prebuilt](./prebuilt) for details, including verifying BUILD files in CI with `with_check`.

## Developing

To release, just press the button on
https://github.com/aspect-build/aspect-gazelle/actions/workflows/release-prebuilt.yaml
