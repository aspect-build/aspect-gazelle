# Aspect Orion - BUILD generation

A set of [Gazelle](https://github.com/bazelbuild/bazel-gazelle) `Language` implementations, utilities, and enhancements for BUILD generation.

## Gazelle Languages

### JavaScript

See [language/js](./language/js).

### Starlark Extensions

The [language/host](./language/host) package provides a Gazelle `Language` implementation enabling the use Starlark extensions for BUILD generation.

## Gazelle Enhancements

Orion provides a variety of enhancements to Gazelle.

The [Orion runner](./runner) enables these enhancements automatically, otherwise manual setup (including patching Gazelle) is required.

### Gitignore

Support for `.gitignore` when generating BUILD files, enabled by the `# gazelle:gitignore enabled|disabled` directive.

### Caching

File based caching of any file analysis by Gazelle language implementations.

Basic caching can be enabled by setting the `ASPECT_CONFIGURE_CACHE` environment variable to a path (e.g. `~/.cache/orion.cache`) for loading+persisting the cache between Gazelle runs.

Further functionality includes [watchman](https://facebook.github.io/watchman/) and other utilities for Gazelle language implementations.

See the [common/cache](./common/cache)

### `--watch` mode

The [Orion runner](./runner) supports a `--watch` mode that uses [watchman](https://facebook.github.io/watchman/) to monitor the filesystem for changes and regenerate BUILD files as needed. This automatically enables the watchman based caching provided by the [common/cache](./common/cache) package.
