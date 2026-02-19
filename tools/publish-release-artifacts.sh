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

# Zip up all the binaries to reduce the download size. DEBs and RPMs
# aren't included to be easier to work with.
find dist/ -type f \
  -name 'alloy*' -not -name '*.deb' -not -name '*.rpm' -not -name '*.zip' -not -name 'alloy-installer-windows-*.exe' \
  -exec zip -j -m "{}.zip" "{}" \;

# For the Windows installer only, we want to keep the original .exe file and create a zipped copy.
find dist/ -type f \
  -name 'alloy-installer-windows-*.exe' \
  -exec zip -j "{}.zip" "{}" \;

# Package rendered Alloy mixin dashboards as a release artifact.
MIXIN_DASHBOARDS_DIR='operations/alloy-mixin/rendered/dashboards'
MIXIN_DASHBOARDS_ARCHIVE="dist/alloy-mixin-dashboards-${RELEASE_TAG}.zip"

if [ ! -d "${MIXIN_DASHBOARDS_DIR}" ]; then
  echo "Error: expected rendered dashboards in ${MIXIN_DASHBOARDS_DIR}"
  exit 1
fi

shopt -s nullglob
dashboard_files=("${MIXIN_DASHBOARDS_DIR}"/*.json)
shopt -u nullglob
if [ ${#dashboard_files[@]} -eq 0 ]; then
  echo "Error: no rendered dashboard JSON files found in ${MIXIN_DASHBOARDS_DIR}"
  exit 1
fi

pushd operations/alloy-mixin/rendered && zip -r "../../../${MIXIN_DASHBOARDS_ARCHIVE}" dashboards && popd

# Generate SHA256 checksums for all release assets.
pushd dist && sha256sum -- * > SHA256SUMS && popd

# Upload all assets to the existing GitHub release.
gh release upload "${RELEASE_TAG}" dist/* --clobber
