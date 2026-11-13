#!/usr/bin/env bash
#
# This script prepares and uploads release assets to an existing GitHub release.
# It should be run from the root of the repository after dist artifacts have been
# built and signed Windows executables have been placed in the dist directory.
#
# Required environment variables:
#   RELEASE_TAG - The release tag to upload assets to (e.g., v1.0.0)
#   GH_TOKEN    - GitHub token with write access to releases
#
set -euxo pipefail

if [ -z "${RELEASE_TAG:-}" ]; then
  echo "Error: RELEASE_TAG environment variable is required"
  exit 1
fi

# Disable xtrace to avoid leaking GH_TOKEN in logs
set +x
if [ -z "${GH_TOKEN:-}" ]; then
  echo "Error: GH_TOKEN environment variable is required"
  exit 1
fi
# Re-enable xtrace
set -x

# Verify rendered Alloy mixin dashboards archive exists.
MIXIN_DASHBOARDS_ARCHIVE="dist/alloy-mixin-dashboards-${RELEASE_TAG}.zip"

if [ ! -f "${MIXIN_DASHBOARDS_ARCHIVE}" ]; then
  echo "Error: expected mixin dashboards archive ${MIXIN_DASHBOARDS_ARCHIVE}. Run 'RELEASE_TAG=${RELEASE_TAG} make dist-alloy-mixin-zip' first."
  exit 1
fi

# Zip up all the binaries to reduce the download size. DEBs and RPMs
# aren't included to be easier to work with.
find dist/ -type f \
  -name 'alloy*' -not -name '*.deb' -not -name '*.rpm' -not -name '*.zip' -not -name 'alloy-installer-windows-*.exe' \
  -exec zip -j -m "{}.zip" "{}" \;

# For the Windows installer only, we want to keep the original .exe file and create a zipped copy.
find dist/ -type f \
  -name 'alloy-installer-windows-*.exe' \
  -exec zip -j "{}.zip" "{}" \;

# Generate a static SPDX SBOM for the release dist tree (issue #4307).
# Prefer syft when available in the build image; fall back to gh/syft install is not required
# for local dry-runs of this script if syft is missing (release CI image should ship or install it).
SBOM_FILE="dist/alloy-${RELEASE_TAG}.spdx.json"
if command -v syft >/dev/null 2>&1; then
  syft dir:dist -o spdx-json="${SBOM_FILE}"
elif command -v docker >/dev/null 2>&1; then
  # Pin a known syft image for reproducible SBOMs when the host has no syft binary.
  docker run --rm -v "${PWD}/dist:/dist" anchore/syft:v1.27.1 \
    dir:/dist -o spdx-json=/dist/"alloy-${RELEASE_TAG}.spdx.json"
else
  echo "Error: syft (or docker to run anchore/syft) is required to generate the release SBOM"
  exit 1
fi

if [ ! -f "${SBOM_FILE}" ]; then
  echo "Error: expected SBOM at ${SBOM_FILE}"
  exit 1
fi

# Generate SHA256 checksums for all release assets (includes SBOM).
pushd dist && sha256sum -- * > SHA256SUMS && popd

# Upload all assets to the existing GitHub release.
gh release upload "${RELEASE_TAG}" dist/* --clobber
