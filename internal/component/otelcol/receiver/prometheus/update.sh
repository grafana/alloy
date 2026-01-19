#!/usr/bin/env bash

# update.sh - Fetches the latest prometheusreceiver internal package from
# opentelemetry-collector-contrib using sparse checkout.
#
# Usage: ./update.sh [branch]
#   branch: The branch/tag to fetch from (default: main)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INTERNAL_DIR="${SCRIPT_DIR}/internal"
PATCHES_DIR="${SCRIPT_DIR}/patches"
BRANCH="${1:-main}"

# Find repo root (where .git is)
REPO_ROOT="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel)"

UPSTREAM_REPO="https://github.com/open-telemetry/opentelemetry-collector-contrib.git"
UPSTREAM_PATH="receiver/prometheusreceiver/internal"

echo "==> Updating prometheusreceiver internal package from upstream"
echo "    Branch: ${BRANCH}"
echo ""

# Create temp directory for clone
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

echo "==> Cloning upstream with sparse checkout..."
cd "${TEMP_DIR}"
git clone --depth 1 --filter=blob:none --sparse --branch "${BRANCH}" "${UPSTREAM_REPO}" repo
cd repo
git sparse-checkout set "${UPSTREAM_PATH}"

echo "==> Replacing local internal directory..."
rm -rf "${INTERNAL_DIR}"
cp -r "${UPSTREAM_PATH}" "${INTERNAL_DIR}"

# Apply patches from repo root (patches should use full repo paths)
if [[ -d "${PATCHES_DIR}" ]] && ls "${PATCHES_DIR}"/*.patch &>/dev/null; then
    echo ""
    echo "==> Applying patches from ${PATCHES_DIR}..."
    cd "${REPO_ROOT}"
    for patch in "${PATCHES_DIR}"/*.patch; do
        [[ -f "${patch}" ]] || continue
        echo "    Applying $(basename "${patch}")..."
        patch -p1 --no-backup-if-mismatch -f < "${patch}" || {
            echo "    ERROR: Failed to apply $(basename "${patch}")"
            echo "    You may need to regenerate this patch for the new upstream version"
            exit 1
        }
    done
else
    echo ""
    echo "==> No patches found, skipping"
fi

echo ""
echo "==> Done!"
echo "    To generate a patch from your changes:"
echo "      git diff -- internal/component/otelcol/receiver/prometheus/internal/ > patches/001-description.patch"
