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

aspect_gazelle(languages = ["js"])
```

## How it works

`aspect_gazelle()` generates a bash wrapper script that invokes the prebuilt binary
resolved through Bazel's toolchain mechanism:

```
aspect_gazelle()
  └─ aspect_gazelle_runner rule (rules.bzl)
       └─ toolchain resolution (@aspect_gazelle_prebuilt//toolchain:type)
            └─ platform binary downloaded via http_file
```

The `MODULE.bazel` registers platform-specific toolchains for `linux_amd64`,
`linux_arm64`, `darwin_amd64`, and `darwin_arm64`. Bazel selects the correct one
at build time based on the exec platform.

## rules_go: `go_deps` must still come from `@gazelle`

If your project uses `rules_go`, you must still declare the `go_deps` extension
from the upstream `@gazelle` module — **not** from `@aspect_gazelle_prebuilt`.
You also need to pass `repo_config` to `aspect_gazelle()` so the Go resolver can
map import paths to external repos; without it, Gazelle falls back to network
lookups against the Go module proxy for every unknown import and appears to hang.

```starlark
# MODULE.bazel
bazel_dep(name = "aspect_gazelle_prebuilt", version = "...") # for BUILD generation
bazel_dep(name = "gazelle", version = "0.51.0")  # for go_deps

go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//:go.mod")
use_repo(
    go_deps,
    "bazel_gazelle_go_repository_config",  # required for aspect_gazelle(repo_config = ...)
    "com_github_some_module",              # add your Go dep repos here
)
```

```starlark
# BUILD.bazel
load("@aspect_gazelle_prebuilt//:def.bzl", "aspect_gazelle")

aspect_gazelle(
    name = "gazelle",
    languages = ["go", "proto"],
    repo_config = "@bazel_gazelle_go_repository_config//:WORKSPACE",
)
```

**Why:** `aspect_gazelle_prebuilt` only wraps the *binary* — it replaces building
the gazelle runner from source with a prebuilt download. `go_deps` is a completely
separate concern: a bzlmod module extension that runs at dependency-resolution time
to turn your `go.mod`/`go.sum` into Bazel external repositories. That logic lives
entirely inside the `@gazelle` module, and `aspect_gazelle_prebuilt` has no reason
to intercept it.

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
| `def.bzl` | forwarding stub → generated from `runner/def.bzl` with `@aspect_gazelle_runner` replaced by `@aspect_gazelle_prebuilt` |

That patched archive is what BCR downloads — the GitHub repository itself retains
only the placeholder values. After the GitHub Release is published,
`publish-to-bcr` opens a PR against the Bazel Central Registry automatically.
