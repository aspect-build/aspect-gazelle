# Gazelle Caching Utilities

This package provides utilities for caching within or across Gazelle invocations.

File-based caching on the aspect-gazelle binary can be enabled either by flag or by env var:

- `--cache` or `--cache=disk` — persists to a file and invalidates entries on content-hash changes.
- `--cache=watchman` — persists to a file and invalidates entries via filesystem events from [watchman](https://facebook.github.io/watchman/) (more efficient on large trees; requires `watchman` on `PATH`).
- `ASPECT_GAZELLE_CACHE=<path>` — sets the cache file location and, when no `--cache` flag is given, implies `--cache=disk`.

When invoked as a `--watch` protocol client the runner automatically installs a watch-optimized disk cache — no flag needed — and invalidates entries based on the watch protocol's change notifications.

The cache file location defaults to `$TMPDIR/aspect-gazelle-<repo>.cache`; set `ASPECT_GAZELLE_CACHE` to override (e.g. `.cache/aspect-gazelle.cache`). The on-disk format is shared between `--cache=disk` and the watch-mode cache, so entries survive mode switches across runs.

## Usage

Gazelle language implementations can use `cache.Get(config.Config)` to fetch a `cache.Cache` implementation for the current invocation. The cache implementation may be a no-op cache if caching is disabled, an in-memory cache that lasts for the duration of the Gazelle invocation, or a file-based cache that persists between Gazelle invocations. Cache invalidation may be handled based on file content hashes, or a more efficient approach such as a [watchman](https://facebook.github.io/watchman/) based cache that invalidates based on filesystem events.

## Setup

The `cache.NewConfigurer()` Gazelle `config.Configurer` must be added to your Gazelle setup. This is done by the [Aspect runner](../../runner) automatically, otherwise must be patched into Gazelle or manually added another way.

The primary utility is a file-based cache that can be used to store and retrieve arbitrary data associated with specific keys. The cache is designed to be efficient and easy to use, with support for automatic serialization and deserialization of data.
