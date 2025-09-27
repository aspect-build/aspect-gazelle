# Gazelle Caching Utilities

This package provides utilities for caching within or across Gazelle invocations.

File based caching is enabled by setting `ASPECT_CONFIGURE_CACHE` to a path (e.g. `.cache/orion.cache`) for loading+persisting the cache between Gazelle runs.

## Usage

Gazelle language implementations can use `cache.Get(config.Config)` to fetch a `cache.Cache` implementation for the current invocation. The cache implementation may be a no-op cache if caching is disabled, an in-memory cache that lasts for the duration of the Gazelle invocation, or a file-based cache that persists between Gazelle invocations. Cache invalidation may be handled based on file content hashes, or a more efficient approach such as a [watchman](https://facebook.github.io/watchman/) based cache that invalidates based on filesystem events.

## Setup

The `cache.NewConfigurer()` Gazelle `config.Configurer` must be added to your Gazelle setup. This is done by the [Orion runner](../../runner) automatically, otherwise must be patched into Gazelle or manually added another way.

The primary utility is a file-based cache that can be used to store and retrieve arbitrary data associated with specific keys. The cache is designed to be efficient and easy to use, with support for automatic serialization and deserialization of data.
