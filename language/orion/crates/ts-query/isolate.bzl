"""Rename the tree-sitter C runtime symbols inside the cgo-linked static library.

The `tree-sitter` crate statically bundles the tree-sitter C runtime, so the
`ts-query` static library exports the full `ts_*` / `tree_sitter_*` C ABI. The
gazelle binary that links this library ALSO links smacker's `go-tree-sitter`
(pulled in by the kotlin extension and by `@rules_python_gazelle_plugin`), which
bundles its OWN copy of that same runtime -- so the final cgo link fails with
"duplicate symbol: ts_language_version" (and ~hundreds more).

This rule rewrites every defined `ts_*` / `tree_sitter_*` symbol in the archive
to an `orion_`-prefixed name -- except the two FFI entry points the Go side
actually calls (`ts_query_run` / `ts_query_free`). `llvm-objcopy --redefine-syms`
renames definitions AND intra-archive references consistently, so orion's own
code still binds to its (renamed) runtime while smacker's unprefixed copy is left
alone: no collision, and the two runtimes stay independent (which is required --
they are different tree-sitter versions and must not be coalesced).

Renaming (not localizing) is deliberate: localizing the symbols would break
orion's cross-object references to them (the linker won't satisfy an external
reference from a now-local definition, and would silently bind it to smacker's
copy instead). And a partial `ld -r` merge isn't an option -- ld64.lld doesn't
implement `-r` for Mach-O.
"""

load("@bazel_tools//tools/cpp:toolchain_utils.bzl", "find_cpp_toolchain")

# Load CcInfo / cc_common explicitly rather than relying on the legacy top-level
# globals: newer Bazel (with autoloads disabled) removes them, which fails in
# consumer workspaces that build orion as an external module.
load("@rules_cc//cc/common:cc_common.bzl", "cc_common")
load("@rules_cc//cc/common:cc_info.bzl", "CcInfo")

# Keep the two FFI entry points (query/query.go calls them); rename every other
# defined `ts_*` / `tree_sitter_*` global. The name is matched after stripping
# leading underscores -- so this catches Mach-O's `_ts_*`, ELF's `ts_*`, and
# source-level-underscored helpers like `_ts_dup` (Mach-O `__ts_dup`) alike --
# and the original leading underscores are preserved on the renamed symbol.
_RENAME_SCRIPT = """
set -eu
NM="$1"; OBJCOPY="$2"; IN="$3"; OUT="$4"
MAP="$(mktemp)"
"$NM" --extern-only --defined-only "$IN" 2>/dev/null | awk '
  { s = $NF; t = s; sub(/^_+/, "", t); lead = s; sub(/[^_].*/, "", lead) }
  (t ~ /^ts_/ || t ~ /^tree_sitter_/) && t != "ts_query_run" && t != "ts_query_free" {
    print s, lead "orion_" t
  }
' | sort -u > "$MAP"
"$OBJCOPY" --redefine-syms="$MAP" "$IN" "$OUT"
"""

def _isolate_ts_symbols_impl(ctx):
    cc_toolchain = find_cpp_toolchain(ctx)
    feature_configuration = cc_common.configure_features(
        ctx = ctx,
        cc_toolchain = cc_toolchain,
        requested_features = ctx.features,
        unsupported_features = ctx.disabled_features,
    )

    linking_context = ctx.attr.src[CcInfo].linking_context

    # The rust_static_library produces exactly one archive; find it.
    in_archive = None
    for li in linking_context.linker_inputs.to_list():
        for lib in li.libraries:
            archive = lib.static_library or lib.pic_static_library
            if archive:
                if in_archive:
                    fail("expected exactly one static library in %s" % ctx.attr.src.label)
                in_archive = archive
    if not in_archive:
        fail("no static library found in %s" % ctx.attr.src.label)

    out_archive = ctx.actions.declare_file("lib%s.a" % ctx.label.name)

    ctx.actions.run_shell(
        inputs = [in_archive],
        outputs = [out_archive],
        tools = [ctx.file._nm, ctx.file._objcopy],
        mnemonic = "IsolateTsSymbols",
        progress_message = "Isolating tree-sitter symbols in %{output}",
        command = _RENAME_SCRIPT,
        arguments = [
            ctx.file._nm.path,
            ctx.file._objcopy.path,
            in_archive.path,
            out_archive.path,
        ],
    )

    # Rebuild the linking context with the original archive swapped for the
    # rewritten one, preserving every other library, link flag and input
    # (e.g. the system libs rustc requires).
    new_linker_inputs = []
    for li in linking_context.linker_inputs.to_list():
        new_libs = []
        for lib in li.libraries:
            archive = lib.static_library or lib.pic_static_library
            if archive == in_archive:
                new_libs.append(cc_common.create_library_to_link(
                    actions = ctx.actions,
                    feature_configuration = feature_configuration,
                    cc_toolchain = cc_toolchain,
                    static_library = out_archive if lib.static_library else None,
                    pic_static_library = out_archive if lib.pic_static_library else None,
                    alwayslink = lib.alwayslink,
                ))
            else:
                new_libs.append(lib)
        new_linker_inputs.append(cc_common.create_linker_input(
            owner = li.owner,
            libraries = depset(new_libs),
            user_link_flags = li.user_link_flags,
            additional_inputs = depset(li.additional_inputs),
        ))

    return [CcInfo(
        compilation_context = ctx.attr.src[CcInfo].compilation_context,
        linking_context = cc_common.create_linking_context(
            linker_inputs = depset(new_linker_inputs),
        ),
    )]

isolate_ts_symbols = rule(
    implementation = _isolate_ts_symbols_impl,
    doc = "Rewrites the tree-sitter C runtime symbols of `src`'s static library " +
          "to an orion_-prefixed name (except ts_query_run/ts_query_free), so it " +
          "can be cgo-linked alongside smacker's go-tree-sitter without collision.",
    attrs = {
        "src": attr.label(
            providers = [CcInfo],
            mandatory = True,
            doc = "A rust_static_library (or other CcInfo) with exactly one archive.",
        ),
        # Source the tools from @llvm (which orion's MODULE depends on) rather
        # than the resolved cc toolchain: a consumer (e.g. aspect-cli-legacy) may
        # build orion with a non-LLVM cc toolchain that has no llvm-objcopy. The
        # @llvm//tools:* aliases resolve to the right binary for the exec platform.
        "_objcopy": attr.label(
            default = "@llvm//tools:llvm-objcopy",
            cfg = "exec",
            allow_single_file = True,
        ),
        "_nm": attr.label(
            default = "@llvm//tools:llvm-nm",
            cfg = "exec",
            allow_single_file = True,
        ),
        "_cc_toolchain": attr.label(default = "@bazel_tools//tools/cpp:current_cc_toolchain"),
    },
    toolchains = ["@bazel_tools//tools/cpp:toolchain_type"],
    fragments = ["cpp"],
)
