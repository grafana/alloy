#!/usr/bin/env bash
set -euo pipefail

# Check if COLLECTOR_PATH is set
if [ -z "${COLLECTOR_PATH:-}" ]; then
  echo "Error: COLLECTOR_PATH is not set"
  exit 1
fi

echo "Setting the local Collector to a local checkout at $COLLECTOR_PATH"

# Replace collector module dependencies with local paths
go list -m -u all | grep 'go\.opentelemetry\.io/collector' | while read -r line; do
  MODULE=$(echo "$line" | awk '{print $1}')
  REL_PATH=${MODULE#go.opentelemetry.io/collector/}
  LOCAL_PATH="$COLLECTOR_PATH/$REL_PATH"
  echo "Replacing $MODULE => $LOCAL_PATH"
  go mod edit -replace="$MODULE=$LOCAL_PATH"
done
