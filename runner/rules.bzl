"""Thin wrapper around upstream gazelle(). Keeps attr shape aligned with //prebuilt/rules.bzl."""

load("@gazelle//:def.bzl", "gazelle")

_GAZELLE_BINARY = Label("//:gazelle_prebuilt_bin")

_VALID_COMMANDS = ["update", "fix"]
_VALID_MODES = ["fix", "diff"]

def aspect_gazelle_runner(
        name,
        command = "update",
        mode = "fix",
        extra_args = [],
        data = [],
        env = {},
        repo_config = None,
        **kwargs):
    """Internal wrapper around upstream gazelle(); public surface is //runner/def.bzl:aspect_gazelle.

    Args:
        name: Target name.
        command: Gazelle subcommand.
        mode: Gazelle mode.
        extra_args: Forwarded to the gazelle binary.
        data: Additional runtime files.
        env: Environment variables. Values are not location-expanded here (unlike //prebuilt/rules.bzl).
        repo_config: Optional repo config file.
        **kwargs: Forwarded to the underlying target.
    """
    if command not in _VALID_COMMANDS:
        fail("Invalid 'command' %r. Must be one of: %s" % (command, ", ".join(_VALID_COMMANDS)))
    if mode not in _VALID_MODES:
        fail("Invalid 'mode' %r. Must be one of: %s" % (mode, ", ".join(_VALID_MODES)))

    if repo_config:
        extra_args = ["-repo_config=$(location %s)" % repo_config] + extra_args
        data = [repo_config] + data

    gazelle(
        name = name,
        gazelle = _GAZELLE_BINARY,
        command = command,
        mode = mode,
        extra_args = extra_args,
        data = data,
        env = env,
        **kwargs
    )
