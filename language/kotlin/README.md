# Kotlin Gazelle Extension

This is a [Gazelle](https://github.com/bazelbuild/bazel-gazelle) `Language` implementation for Kotlin using the [rules_kotlin](https://github.com/bazelbuild/rules_kotlin) `jvm` rules.

> [!NOTE]
> The `aspect_gazelle_kotlin` module is only needed when **building the Gazelle binary from source**. If you use [`aspect_gazelle_prebuilt`](../../prebuilt/), this language is already compiled into the prebuilt binary and no other `aspect_gazelle_*` module is required.

This was originally implemented by @jbedard in https://github.com/aspect-build/aspect-cli
and has been separated out to a standalone git repository and Bazel module.

The work was sponsored by @reddaly and the GoogleX Tapestry team, thanks so much!
