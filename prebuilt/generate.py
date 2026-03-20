#!/usr/bin/env python3
"""Generates prebuilt/ files from templates and runner sources.

Usage: generate.py <module_template> <tag> <checksums_file> <runner_def_bzl>

  module_template:  path to prebuilt/MODULE.bazel.tmpl
  tag:              release tag, e.g. "2024.12.3+abc1234"
  checksums_file:   path to CHECKSUMS file produced by shasum -a 256
  runner_def_bzl:   path to runner/def.bzl

Writes:
  prebuilt/MODULE.bazel  (module_template with .tmpl stripped)
  prebuilt/def.bzl       (runner_def_bzl patched for prebuilt)
"""
import re
import sys
from pathlib import Path

module_template, tag, checksums_file, runner_def_bzl = sys.argv[1:5]

module_out = module_template.removesuffix(".tmpl")
prebuilt_def_bzl = str(Path(module_template).parent / "def.bzl")

# --- MODULE.bazel ---

# Parse CHECKSUMS: "<sha256>  assets/aspect_gazelle-<platform>"
shas = {}
with open(checksums_file) as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        sha, path = line.split()
        platform = path.split("aspect_gazelle-")[1]  # e.g. "linux_amd64"
        shas[platform] = sha

content = open(module_template).read()

# BCR requires a clean semver version without build metadata (+hash)
module_version = tag.split("+")[0]

# Stamp version in module() call (strip build metadata for BCR compatibility)
content = re.sub(r'version = "0\.0\.0"', f'version = "{module_version}"', content)

# Stamp full tag in download URLs
content = content.replace("/0.0.0/", f"/{tag}/")

# Stamp sha256 for each platform
for platform in ["linux_amd64", "linux_arm64", "darwin_arm64"]:
    sha = shas.get(platform, "")
    if not sha:
        print(f"ERROR: no sha256 found for {platform}", file=sys.stderr)
        sys.exit(1)
    repo = f"aspect_gazelle_prebuilt_{platform}"
    content = re.sub(
        rf'(name = "{re.escape(repo)}".*?sha256 = ")[^"]*(")',
        rf'\g<1>{sha}\g<2>',
        content,
        flags=re.DOTALL,
    )

open(module_out, "w").write(content)
print(f"Generated {module_out} from {module_template} with version={tag}")

# --- def.bzl ---

# Copy runner/def.bzl and patch it for the prebuilt module.
content = open(runner_def_bzl).read()

content = (
    "# Generated from runner/def.bzl by the release process. Do not edit manually.\n"
    + content
)

content = content.replace("@aspect_gazelle_runner", "@aspect_gazelle_prebuilt")

open(prebuilt_def_bzl, "w").write(content)
print(f"Generated {prebuilt_def_bzl} from {runner_def_bzl}")
