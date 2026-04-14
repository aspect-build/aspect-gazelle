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

The `MODULE.bazel` registers platform-specific toolchains for `linux_amd64`,
`linux_arm64`, `darwin_amd64`, and `darwin_arm64`. Bazel selects the correct one
at build time based on the exec platform.

## How releases work

To cut a release, push a scoped semver tag:

```bash
git tag prebuilt-v1.2.3
git push origin prebuilt-v1.2.3
```

The `release.yaml` workflow triggers on `prebuilt-v*.*.*` tags. Pre-release tags
(with a `-` in the version part, e.g. `prebuilt-v1.0.0-alpha1`) create a GitHub
pre-release and skip BCR publishing.

The release pipeline (`release_prep.sh`) builds the gazelle binary for each platform,
then produces a patched source archive where placeholder files are replaced with
real values:

| File | Placeholder → Release value |
|------|-----------------------------|
| `integrity.bzl` | zeroed sha256s and `0.0.0` tag → real sha256s and full release tag |
| `MODULE.bazel` | `0.0.0` → stripped version e.g. `2026.12.3` |
| `def.bzl` | forwarding stub → full copy of `runner/def.bzl` with `@aspect_gazelle_runner` replaced by `@aspect_gazelle_prebuilt` |

That patched archive is what BCR downloads — the GitHub repository itself retains
only the placeholder values. After the GitHub Release is published,
`publish-to-bcr` opens a PR against the Bazel Central Registry automatically.
