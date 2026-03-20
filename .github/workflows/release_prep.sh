#!/usr/bin/env bash
# Prepares a release archive with integrity hashes and generated files patched in.
# Called by release_ruleset.yaml as: release_prep.sh <tag>
# Writes release notes to stdout.
set -o errexit -o nounset -o pipefail

TAG=${1}

if [[ "$TAG" != prebuilt-* ]]; then
	echo "Unknown tag format: ${TAG}" >&2
	exit 1
fi

MODULE_VERSION="${TAG#prebuilt-v}"
PREFIX="aspect-gazelle-${TAG}"
ARCHIVE="aspect-gazelle-${TAG}.tar.gz"
PLATFORMS=(linux_amd64 linux_arm64 darwin_amd64 darwin_arm64)
ARCHIVE_TMP=$(mktemp)

git archive --format=tar --prefix="${PREFIX}/" "${TAG}" >"$ARCHIVE_TMP"

# ---- Build prebuilt binaries ----
mkdir -p artifacts
for platform in "${PLATFORMS[@]}"; do
	(cd runner && bazel run \
		--run_under cp \
		--platforms="@rules_go//go/toolchain:${platform}_cgo" \
		//:gazelle_prebuilt_bin \
		"$PWD/../artifacts/aspect_gazelle-${platform}")
done

# Generate sha256 files per binary
for platform in "${PLATFORMS[@]}"; do
	sha256sum "artifacts/aspect_gazelle-${platform}" | awk '{print $1}' >"artifacts/aspect_gazelle-${platform}.sha256"
done

# ---- Patch files in the archive ----
PATCH_DIR=$(mktemp -d)
mkdir -p "${PATCH_DIR}/${PREFIX}/prebuilt"

# integrity.bzl with real hashes and tag
{
	echo '"Generated at release time by release_prep.sh."'
	echo
	echo 'PREBUILT_BINARY_INTEGRITY = {'
	for platform in "${PLATFORMS[@]}"; do
		sha=$(cat "artifacts/aspect_gazelle-${platform}.sha256")
		echo "    \"${platform}\": \"${sha}\","
	done
	echo '}'
	echo "PREBUILT_TAG = \"${TAG}\""
} >"${PATCH_DIR}/${PREFIX}/prebuilt/integrity.bzl"

# MODULE.bazel with real version
sed "s/^    version = \"0\.0\.0\"/    version = \"${MODULE_VERSION}\"/" \
	prebuilt/MODULE.bazel >"${PATCH_DIR}/${PREFIX}/prebuilt/MODULE.bazel"

# def.bzl generated from runner/def.bzl
{
	echo "# Generated from runner/def.bzl by the release process. Do not edit manually."
	sed 's/@aspect_gazelle_runner/@aspect_gazelle_prebuilt/g' runner/def.bzl
} >"${PATCH_DIR}/${PREFIX}/prebuilt/def.bzl"

# Delete placeholder files from the archive and append patched ones
tar --file "$ARCHIVE_TMP" --delete \
	"${PREFIX}/prebuilt/integrity.bzl" \
	"${PREFIX}/prebuilt/MODULE.bazel" \
	"${PREFIX}/prebuilt/def.bzl"

tar --file "$ARCHIVE_TMP" --append \
	-C "$PATCH_DIR" \
	"${PREFIX}/prebuilt/integrity.bzl" \
	"${PREFIX}/prebuilt/MODULE.bazel" \
	"${PREFIX}/prebuilt/def.bzl"

gzip <"$ARCHIVE_TMP" >"$ARCHIVE"

cat <<EOF
Add to your \`MODULE.bazel\`:

\`\`\`starlark
bazel_dep(name = "aspect_gazelle_prebuilt", version = "${MODULE_VERSION}")
\`\`\`

Then in your \`BUILD.bazel\`:

\`\`\`starlark
load("@aspect_gazelle_prebuilt//:def.bzl", "aspect_gazelle")

aspect_gazelle(
    name = "gazelle",
    languages = ["js", "python"],
    extensions = ["//tools/gazelle:my_rule.star"],
)
\`\`\`
EOF
