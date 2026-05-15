# Aspect Gazelle Runner

> **For external use, see [aspect_gazelle_prebuilt](../prebuilt/) which provides a
> prebuilt binary without requiring a Go toolchain.**

This module builds the `aspect_gazelle` binary from source. It is the source of
truth for releases and is intended for contributors and from-source builds.

The binary provides an enhanced version of the `gazelle_binary()` rule with:

- enable/disable languages at runtime instead of at build time
- gitignore support (on by default; opt out with `--gitignore=false`)
- opentelemetry tracing support
- watch protocol support
- caching of gazelle source code analysis
- dx enhancements including:
  - stats outputted to the console
  - progress/status reporting
