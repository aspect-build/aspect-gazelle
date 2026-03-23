#!/usr/bin/env python3
"""Stamps prebuilt/ files with release version and sha256 checksums.

Usage: generate.py <module_bazel> <tag> <checksums_file> <runner_def_bzl>

  module_bazel:     path to prebuilt/MODULE.bazel
  tag:              release tag, e.g. "2024.12.3+abc1234"
  checksums_file:   path to CHECKSUMS file produced by shasum -a 256
  runner_def_bzl:   path to runner/def.bzl

Writes:
  prebuilt/MODULE.bazel    (version stamped in place, build metadata stripped)
  prebuilt/extensions.bzl  (full tag stamped in URLs, sha256 per platform)
  prebuilt/def.bzl         (runner_def_bzl patched for prebuilt)
"""
import re
import sys
from pathlib import Path

module_bazel, tag, checksums_file, runner_def_bzl = sys.argv[1:5]

prebuilt_dir = Path(module_bazel).parent
extensions_bzl = prebuilt_dir / "extensions.bzl"
prebuilt_def_bzl = prebuilt_dir / "def.bzl"

# BCR requires clean semver without build metadata
module_version = tag.split("+")[0]

# --- MODULE.bazel (stamp version in place) ---

content = open(module_bazel).read()
content = re.sub(r'version = "[^"]*"', f'version = "{module_version}"', content, count=1)
open(module_bazel, "w").write(content)
print(f"Stamped version {module_version} in {module_bazel}")

# --- extensions.bzl (stamp full tag in URLs and sha256 per platform) ---

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

for platform in ["linux_amd64", "linux_arm64", "darwin_arm64"]:
    if not shas.get(platform):
        print(f"ERROR: no sha256 found for {platform}", file=sys.stderr)
        sys.exit(1)

content = open(extensions_bzl).read()

# Stamp full tag in download URLs (matches any existing version)
content = re.sub(r'(releases/download/)[^/]+(/)', rf'\g<1>{tag}\g<2>', content)

# Stamp sha256 for each platform
for platform, sha in shas.items():
    content = re.sub(
        rf'("{platform}".*?"sha256": ")[^"]*(")',
        rf'\g<1>{sha}\g<2>',
        content,
        flags=re.DOTALL,
    )

open(extensions_bzl, "w").write(content)
print(f"Stamped {extensions_bzl}")

# --- def.bzl (copy runner/def.bzl and patch for prebuilt) ---

content = open(runner_def_bzl).read()
content = (
    "# Generated from runner/def.bzl by the release process. Do not edit manually.\n"
    + content
)
content = content.replace("@aspect_gazelle_runner", "@aspect_gazelle_prebuilt")
open(str(prebuilt_def_bzl), "w").write(content)
print(f"Generated {prebuilt_def_bzl} from {runner_def_bzl}")
