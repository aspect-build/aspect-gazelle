#!/usr/bin/env bash
# Prepares a release archive with integrity hashes and generated files patched in.
# Called by release_ruleset.yaml as: release_prep.sh <tag>
# Writes release notes to stdout.
set -o errexit -o nounset -o pipefail

TAG=${1}

# ---- Source-module releases (aspect_gazelle_js, _orion, _runner, ...) ----
# These ship no prebuilt binaries; the archive is just the module's subtree
# hoisted to the archive root, with the module version patched in. Add a module
# by adding a "<tag-prefix>*) <module root>" case below (and its .bcr templates).
SRC_MODULE_ROOT=""
case "$TAG" in
js-v*) SRC_MODULE_ROOT="language/js" ;;
	# orion-v*) SRC_MODULE_ROOT="language/orion" ;;  # add when its .bcr/ templates exist
	# runner-v*) SRC_MODULE_ROOT="runner" ;;          # add when its .bcr/ templates exist
esac
if [[ -n "$SRC_MODULE_ROOT" ]]; then
	# Tag carries the module's prefix (e.g. js-v1.2.3), so the archive is
	# aspect-gazelle-js-v1.2.3.tar.gz, matching strip_prefix "aspect-gazelle-{TAG}"
	# in the module's .bcr source.template.json.
	MODULE_VERSION="${TAG#*-v}"
	PREFIX="aspect-gazelle-${TAG}"
	ARCHIVE="aspect-gazelle-${TAG}.tar.gz"

	UNPACK_DIR=$(mktemp -d)
	mkdir -p "${UNPACK_DIR}/${PREFIX}"
	# git archive yields ${PREFIX}/${SRC_MODULE_ROOT}/...; strip the PREFIX + the
	# module-root segments so MODULE.bazel lands at the archive root. Strip count
	# = 1 (PREFIX) + module-root segments = (slashes in root) + 2.
	ROOT_SLASHES=$(tr -cd '/' <<<"$SRC_MODULE_ROOT" | wc -c)
	git archive --format=tar --prefix="${PREFIX}/" "${TAG}" -- "$SRC_MODULE_ROOT" |
		tar --strip-components=$((ROOT_SLASHES + 2)) -xf - -C "${UNPACK_DIR}/${PREFIX}"

	# Patch the module version (0.0.0 -> real).
	sed -i "s/^    version = \"0\.0\.0\"/    version = \"${MODULE_VERSION}\"/" \
		"${UNPACK_DIR}/${PREFIX}/MODULE.bazel"

	# Module name + @llvm pin, surfaced in the consumer setup notes below.
	MODULE_NAME=$(sed -n 's/^    name = "\([^"]*\)",$/\1/p' "${UNPACK_DIR}/${PREFIX}/MODULE.bazel" | head -1)
	LLVM_VERSION=$(sed -n 's/.*bazel_dep(name = "llvm", version = "\([^"]*\)").*/\1/p' \
		"${UNPACK_DIR}/${PREFIX}/MODULE.bazel")

	# The in-repo .bazelrc imports %workspace%/../../tools/*.bazelrc, which don't
	# exist in the standalone archive. Nothing in the BCR flow runs bazel at the
	# module root (the bcr_test_module under e2e/smoke carries its own
	# self-contained .bazelrc), so drop it from the published archive.
	rm -f "${UNPACK_DIR}/${PREFIX}/.bazelrc"

	tar -czf "$ARCHIVE" -C "$UNPACK_DIR" "${PREFIX}"
	rm -rf "$UNPACK_DIR"

	cat <<EOF
Add to your \`MODULE.bazel\`:

\`\`\`starlark
bazel_dep(name = "${MODULE_NAME}", version = "${MODULE_VERSION}")

# The gazelle binary embeds a cgo Rust parser. ${MODULE_NAME} (transitively)
# registers the LLVM + Rust toolchains, but @llvm must be declared so it's
# visible by apparent name for the .bazelrc flags below.
bazel_dep(name = "llvm", version = "${LLVM_VERSION}")
\`\`\`

The cgo parser links against a hermetic LLVM toolchain. These flags configure it
and **cannot** propagate from a dependency module (which is why this module ships
no root \`.bazelrc\`), so add them to your own \`.bazelrc\`:

\`\`\`
common --repo_env=BAZEL_DO_NOT_DETECT_CPP_TOOLCHAIN=1
common --repo_env=BAZEL_NO_APPLE_CPP_TOOLCHAIN=1
common --@llvm//config:experimental_stub_libgcc_s=True
common --linkopt=-no-pie
\`\`\`
EOF
	exit 0
fi

if [[ "$TAG" != prebuilt-* ]]; then
	echo "Unknown tag format: ${TAG}" >&2
	exit 1
fi

MODULE_VERSION="${TAG#prebuilt-v}"
PREFIX="aspect-gazelle-${TAG}"
ARCHIVE="aspect-gazelle-${TAG}.tar.gz"
PLATFORMS=(linux_amd64 linux_arm64 darwin_amd64 darwin_arm64)
ARCHIVE_TMP=$(mktemp)

UNPACK_DIR=$(mktemp -d)
mkdir -p "${UNPACK_DIR}/${PREFIX}"
git archive --format=tar --prefix="${PREFIX}/" "${TAG}" -- prebuilt |
	tar --strip-components=2 -xf - -C "${UNPACK_DIR}/${PREFIX}"
tar -cf "$ARCHIVE_TMP" -C "$UNPACK_DIR" "${PREFIX}"
rm -rf "$UNPACK_DIR"

# ---- Build prebuilt binaries ----
mkdir -p artifacts
for platform in "${PLATFORMS[@]}"; do
	(cd runner && bazel run \
		--run_under cp \
		--config=release \
		"//:gazelle_prebuilt_bin.${platform}" \
		"$PWD/../artifacts/aspect_gazelle-${platform}")
done

# Generate sha256 files per binary
for platform in "${PLATFORMS[@]}"; do
	sha256sum "artifacts/aspect_gazelle-${platform}" | awk '{print $1}' >"artifacts/aspect_gazelle-${platform}.sha256"
done

# ---- Patch files in the archive ----
PATCH_DIR=$(mktemp -d)
mkdir -p "${PATCH_DIR}/${PREFIX}"

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
} >"${PATCH_DIR}/${PREFIX}/integrity.bzl"

# MODULE.bazel with real version
sed "s/^    version = \"0\.0\.0\"/    version = \"${MODULE_VERSION}\"/" \
	prebuilt/MODULE.bazel >"${PATCH_DIR}/${PREFIX}/MODULE.bazel"

# def.bzl generated from runner/def.bzl
{
	echo "# Generated from runner/def.bzl by the release process. Do not edit manually."
	sed 's/@aspect_gazelle_runner/@aspect_gazelle_prebuilt/g' runner/def.bzl
} >"${PATCH_DIR}/${PREFIX}/def.bzl"

# Delete placeholder files from the archive and append patched ones
tar --file "$ARCHIVE_TMP" --delete \
	"${PREFIX}/integrity.bzl" \
	"${PREFIX}/MODULE.bazel" \
	"${PREFIX}/def.bzl"

tar --file "$ARCHIVE_TMP" --append \
	-C "$PATCH_DIR" \
	"${PREFIX}/integrity.bzl" \
	"${PREFIX}/MODULE.bazel" \
	"${PREFIX}/def.bzl"

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
