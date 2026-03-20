"""Platform binaries for aspect_gazelle_prebuilt."""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")
load("//:integrity.bzl", "PREBUILT_BINARY_INTEGRITY", "PREBUILT_TAG")

_BASE_URL = "https://github.com/aspect-build/aspect-gazelle/releases/download/{tag}/aspect_gazelle-{platform}"

def _impl(_module_ctx):
    for platform, sha256 in PREBUILT_BINARY_INTEGRITY.items():
        http_file(
            name = "aspect_gazelle_prebuilt_" + platform,
            executable = True,
            sha256 = sha256,
            urls = [_BASE_URL.format(tag = PREBUILT_TAG, platform = platform)],
        )

prebuilt_extension = module_extension(implementation = _impl)
