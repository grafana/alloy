#!/usr/bin/env bash

# ensure-builder.sh ensures the OCB builder binary is installed at the correct version.
# It checks if builder is available in PATH and automatically installs/upgrades if needed.
#
# Usage: ensure-builder.sh <BUILDER_VERSION>
#
# Arguments:
#   BUILDER_VERSION - Version of the builder required (e.g., v0.139.0)

set -euo pipefail

if [ $# -lt 1 ]; then
	echo "Error: Missing required argument" >&2
	echo "Usage: $0 <BUILDER_VERSION>" >&2
	exit 1
fi

BUILDER_VERSION="$1"

# Check if builder is installed
if command -v builder >/dev/null 2>&1; then
	BUILDER_PATH=$(command -v builder)
	
	# Check version
	if go version -m "$BUILDER_PATH" 2>/dev/null | grep -q "mod.*go.opentelemetry.io/collector/cmd/builder.*$BUILDER_VERSION"; then
		# Version matches, proceed
		echo "Builder found in PATH with correct version ($BUILDER_VERSION)"
		exit 0
	else
		# Version mismatch - automatically upgrade/downgrade
		INSTALLED_VERSION=$(go version -m "$BUILDER_PATH" 2>/dev/null | grep "mod.*go.opentelemetry.io/collector/cmd/builder" | awk '{print $3}' || echo "unknown")
		echo "================================================================================"
		echo "WARNING: Builder version mismatch detected!"
		echo "  Installed version: $INSTALLED_VERSION"
		echo "  Required version:  $BUILDER_VERSION"
		echo ""
		echo "Automatically upgrading/downgrading builder to $BUILDER_VERSION..."
		echo "This will install to: $(go env GOBIN 2>/dev/null || echo "$(go env GOPATH)/bin")"
		echo "================================================================================"
		GOOS= GOARCH= go install "go.opentelemetry.io/collector/cmd/builder@$BUILDER_VERSION"
		echo "Builder $BUILDER_VERSION installed successfully"
	fi
else
	# Builder not installed - automatically install
	echo "================================================================================"
	echo "Builder not found in PATH"
	echo ""
	echo "Installing builder $BUILDER_VERSION..."
	echo "This will install to: $(go env GOBIN 2>/dev/null || echo "$(go env GOPATH)/bin")"
	echo "================================================================================"
	GOOS= GOARCH= go install "go.opentelemetry.io/collector/cmd/builder@$BUILDER_VERSION"
	echo "Builder $BUILDER_VERSION installed successfully"
fi

