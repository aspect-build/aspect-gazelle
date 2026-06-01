"""Always-opt rust_static_library wrapper for the cgo-linked JS parser.

`with_cfg` pins compilation_mode=opt on this target's subtree -- the crate plus
the oxc deps where parse time actually goes -- so the parser is built at
opt-level 3 no matter the consumer's build mode. That matters when
aspect_gazelle_js is used as a source bazel_dep inside someone's
`gazelle_binary(languages = [...])`: their `bazel run //:gazelle` is fastbuild
and we can't rely on `-c opt`, but they still get an optimized parser.

opt-level 3 is the bulk of the win, so it's all we pin here. We deliberately skip
LTO and codegen-units=1: LTO's gain over opt-level 3 is marginal and its cold
build cost is high (aspect-cli skips it too). The prebuilt release additionally
builds with `-c opt`, which optimizes the Go side and strips the final binary.
"""

load("@rules_rs//rs:rust_static_library.bzl", "rust_static_library")
load("@with_cfg.bzl", "with_cfg")

opt_rust_static_library, _opt_rust_static_library_rule = (
    with_cfg(rust_static_library)
        .set("compilation_mode", "opt")
        .build()
)
