# aspect_gazelle_prebuilt

A prebuilt distribution of [aspect_gazelle_runner](../runner/) that downloads
platform-specific binaries from GitHub Releases instead of compiling from source.

## Usage

```starlark
# MODULE.bazel
bazel_dep(name = "aspect_gazelle_prebuilt", version = "...")
```

```starlark
# BUILD.bazel
load("@aspect_gazelle_prebuilt//:def.bzl", "aspect_gazelle")

aspect_gazelle(name = "gazelle")
```

## How it works

`aspect_gazelle()` calls the upstream `gazelle()` macro with a binary label that
resolves through Bazel's toolchain mechanism:

```
aspect_gazelle()
  └─ gazelle(gazelle = "@aspect_gazelle_prebuilt//:gazelle_prebuilt_bin")
       └─ gazelle_runner rule (rules.bzl)
            └─ toolchain resolution (@aspect_gazelle_prebuilt//toolchain:type)
                 └─ platform binary downloaded via http_file
```

The `register_toolchains("//toolchain:all")` call in `MODULE.bazel` registers
platform-specific toolchains for `linux_amd64`, `linux_arm64`, and `darwin_arm64`.
Bazel selects the correct one at build time based on the exec platform.

`def.bzl` is generated from `runner/def.bzl` by the release process. The only
differences are the header comment and `@aspect_gazelle_runner` references replaced
with `@aspect_gazelle_prebuilt`. Do not edit it manually.

## How releases work

1. `runner/` builds the `gazelle_prebuilt_bin` target for each platform.
2. The release workflow stamps the real version, sha256 values, and `def.bzl` into
   `prebuilt/` (which otherwise contains placeholder values), then pushes a release
   tag pointing to that commit.
3. The source tarball at that tag is what BCR downloads — it contains the stamped
   `MODULE.bazel` and `def.bzl`.

`MODULE.bazel` and `def.bzl` are gitignored and not present on the `main` branch —
only `MODULE.bazel.tmpl` is checked in. The generated files exist only on release
tags and should not be edited manually.
