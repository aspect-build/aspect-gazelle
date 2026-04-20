"""
Aspect enhanced Gazelle
"""

load("//:rules.bzl", "aspect_gazelle_runner")

# Keep in sync with the bazel-gazelle DEFAULT_LANGUAGES:
# https://github.com/bazel-contrib/bazel-gazelle/blob/v0.50.0/def.bzl#L59-L63
# Not used internally — the aspect_gazelle() macro's default language set is
# controlled by the runner binary (runner/bin/gazelle/languages.go).
DEFAULT_LANGUAGES = [
    "visibility_extension",
    "proto",
    "go",
]

# Keep in sync with the switch statement in runner/runner.go AddLanguage()
_VALID_LANGUAGES = [
    "buf",
    "cc",
    "go",
    "js",
    "kotlin",
    "orion",
    "proto",
    "python",
    "starlark",
    "visibility_extension",
]

def aspect_gazelle(
        name = "gazelle",
        languages = [],
        extensions = [],
        extra_args = [],
        with_check = False,
        **kwargs):
    """Creates a Gazelle target for BUILD generation.

    Several well-supported languages are built into the prebuilt binary, and a subset
    of them (Go, Protobuf, Python, JavaScript, Starlark, and a few others) are enabled
    by default. Use the `languages` argument to explicitly select which to enable, or
    pass `DEFAULT_LANGUAGES` for the same set as Gazelle's built-in default.

    Aspect Orion extensions are Starlark-based plugins that provide additional BUILD
    file generation capabilities beyond the standard language extensions. These can be
    added via the `extensions` argument.

    The underlying `gazelle()` binary and the `mode` attribute are managed by this
    macro and cannot be overridden: the main target runs in `mode = "fix"`, and the
    optional `.check` target (see `with_check`) runs in `mode = "diff"`.

    Example:
        ```starlark
        load("@aspect_gazelle_runner//:def.bzl", "aspect_gazelle")

        # Default name "gazelle", default languages, plus a `:gazelle.check` target for CI.
        aspect_gazelle(with_check = True)

        # Enable only specific languages
        aspect_gazelle(
            name = "gazelle_go_proto",
            languages = ["go", "proto"],
        )

        # Add Orion extensions for custom generation
        aspect_gazelle(
            name = "gazelle_with_orion",
            extensions = ["//tools/gazelle:my_extension.axl"],
        )
        ```

    Args:
        name: Name of the target. Defaults to "gazelle".
        languages: A list of Gazelle language string keys to enable. If empty (default),
            a default subset of the built-in languages is enabled. Examples:
            ["go", "proto", "python"].
        extensions: A list of labels pointing to Aspect Gazelle Orion Starlark extensions
            to load. These extensions provide additional BUILD file generation logic.
        extra_args: Additional command-line arguments passed to Gazelle.
        with_check: If True, also creates a `<name>.check` target that runs Gazelle in
            `diff` mode, suitable for CI to verify BUILD files are up to date.
        **kwargs: Standard Bazel rule attributes (e.g. `visibility`, `testonly`).
    """

    for lang in languages:
        if lang not in _VALID_LANGUAGES:
            fail("Invalid language %r in 'languages'. Valid languages are: %s" % (lang, ", ".join(_VALID_LANGUAGES)))

    command = kwargs.pop("command", "update")
    if command not in ("update", "fix"):
        fail("Invalid 'command' %r. Must be one of: \"update\", \"fix\"." % command)

    common = dict(
        command = command,
        extra_args = extra_args,
        env = kwargs.pop("env", {}) | {
            "ENABLE_LANGUAGES": ",".join(languages),
            "ORION_EXTENSIONS": ",".join(["$(rootpath %s)" % p for p in extensions]),
        },
        data = kwargs.pop("data", []) + extensions,
        tags = kwargs.pop("tags", []) + ["manual", "supports_incremental_build_protocol"],
        repo_config = kwargs.pop("repo_config", None),
    )
    if "visibility" in kwargs:
        common["visibility"] = kwargs.pop("visibility")
    if "testonly" in kwargs:
        common["testonly"] = kwargs.pop("testonly")

    if kwargs:
        fail("aspect_gazelle() got unexpected keyword argument(s): %s" % ", ".join(kwargs.keys()))

    aspect_gazelle_runner(
        name = name,
        mode = "fix",
        **common
    )

    if with_check:
        aspect_gazelle_runner(
            name = name + ".check",
            mode = "diff",
            **common
        )
