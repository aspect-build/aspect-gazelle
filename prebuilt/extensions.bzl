"""Platform binaries for aspect_gazelle_prebuilt."""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

_PLATFORMS = {
    "linux_amd64": {
        "sha256": "e2d84bcb9e4595fc2d40809eeab5c7fb8b891963195ab58db3f9456ddb5fed32",
        "url": "https://github.com/aspect-build/aspect-gazelle/releases/download/2026.12.17+5a258332/aspect_gazelle-linux_amd64",
    },
    "linux_arm64": {
        "sha256": "db8db55db95b98bac6c147cf96a26f7430f95b6c4761828544d8828eab400429",
        "url": "https://github.com/aspect-build/aspect-gazelle/releases/download/2026.12.17+5a258332/aspect_gazelle-linux_arm64",
    },
    "darwin_arm64": {
        "sha256": "b931f5b1d83cde28015299d038ed2a84fb1d4824aac456a0a4b9648e44503e01",
        "url": "https://github.com/aspect-build/aspect-gazelle/releases/download/2026.12.17+5a258332/aspect_gazelle-darwin_arm64",
    },
}

def _impl(_module_ctx):
    for platform, info in _PLATFORMS.items():
        http_file(
            name = "aspect_gazelle_prebuilt_" + platform,
            executable = True,
            sha256 = info["sha256"],
            url = info["url"],
        )

prebuilt_extension = module_extension(implementation = _impl)
