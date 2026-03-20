"""Toolchain definitions for aspect_gazelle_prebuilt."""

GazellePrebuiltInfo = provider(
    doc = "Information about a prebuilt gazelle binary.",
    fields = {"binary": "The prebuilt executable file"},
)

def _gazelle_prebuilt_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo(
        gazelle_info = GazellePrebuiltInfo(binary = ctx.executable.binary),
    )]

gazelle_prebuilt_toolchain = rule(
    implementation = _gazelle_prebuilt_toolchain_impl,
    attrs = {
        "binary": attr.label(
            allow_single_file = True,
            executable = True,
            cfg = "exec",
            mandatory = True,
        ),
    },
)
