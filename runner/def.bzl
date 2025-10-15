"""
Aspect enhanced Gazelle
"""

load("@gazelle//:def.bzl", "gazelle")

def aspect_gazelle(languages = [], extensions = [], **kwargs):
    """Creates a Gazelle target for BUILD file generation and update.

    This macro provides an enhanced version of the standard `gazelle()` macro that:
    - Bundles multiple well-supported language extensions into a single binary
    - Supports Aspect Orion extensions for BUILD file generation via starlark extensions

    Standard well-supported languages are built into the binary and enabled by default.
    These include common languages like Go, Protobuf, and others. Use the `languages`
    argument to enable only a specific subset if desired.

    Aspect Orion extensions are Starlark-based plugins that provide additional BUILD
    file generation capabilities beyond the standard language extensions. These can be
    added via the `extensions` argument.

    Example:
        ```starlark
        load("@aspect_gazelle_runner//:def.bzl", "aspect_gazelle")

        # Basic usage with all default languages
        aspect_gazelle(
            name = "gazelle",
        )

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

        # Update all BUILD files
        aspect_gazelle(
            name = "gazelle_update",
            command = "fix",
        )
        ```

    Args:
        languages: A list of Gazelle language string keys to enable. If empty (default),
            all built-in languages are enabled. Examples: ["go", "proto", "python"].
        extensions: A list of labels pointing to Aspect Gazelle Orion Starlark extensions
            to load. These extensions provide additional BUILD file generation logic.
        **kwargs: Additional arguments passed directly to the underlying `gazelle()` macro including:
            - `command`: The Gazelle command to run (e.g., "update", "fix")
            - `mode`: The Gazelle mode (e.g., "diff", "update", "fix")
            - `args`: Additional command-line arguments for Gazelle
    """

    gazelle(
        gazelle = Label("@aspect_gazelle_runner//bin/gazelle:gazelle"),
        env = kwargs.pop("env", {}) | {
            "ENABLE_LANGUAGES": ",".join(languages),
            "ORION_EXTENSIONS": ",".join(["$(rootpath %s)" % p for p in extensions]),
        },
        data = kwargs.pop("data", []) + extensions,
        tags = kwargs.pop("tags", []) + ["supports_incremental_build_protocol"],
        **kwargs
    )
