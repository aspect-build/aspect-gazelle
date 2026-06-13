load("@bazel_skylib//rules:build_test.bzl", "build_test")
load("@bazelrc-preset.bzl", "bazelrc_preset")

bazelrc_preset(name = "preset")

# Ensures the nogo config and every analyzer package compiles in the root
# workspace. Lives here (not in //tools/nogo) because bazel_skylib is a dev
# dep, and //tools/nogo is loaded by the sibling workspaces.
build_test(
    name = "nogo_build_test",
    targets = ["//tools/nogo"],
)
