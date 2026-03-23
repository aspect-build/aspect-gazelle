"""Rules for aspect_gazelle_prebuilt."""

def _gazelle_runner_impl(ctx):
    toolchain = ctx.toolchains["//toolchain:type"]
    binary = toolchain.gazelle_info.binary
    out = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(output = out, target_file = binary, is_executable = True)
    return [DefaultInfo(executable = out, files = depset([out]))]

gazelle_runner = rule(
    implementation = _gazelle_runner_impl,
    executable = True,
    toolchains = ["//toolchain:type"],
)
