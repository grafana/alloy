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
  -name 'alloy*' -not -name '*.deb' -not -name '*.rpm' -not -name 'alloy-installer-windows-*.exe' \
  -exec zip -j -m "{}.zip" "{}" \;

# For the Windows installer only, we want to keep the original .exe file and create a zipped copy.
find dist/ -type f \
  -name 'alloy-installer-windows-*.exe' \
  -exec zip -j "{}.zip" "{}" \;

# Generate SHA256 checksums for all release assets.
pushd dist && sha256sum -- * > SHA256SUMS && popd

# Upload all assets to the existing GitHub release.
gh release upload "${RELEASE_TAG}" dist/* --clobber
