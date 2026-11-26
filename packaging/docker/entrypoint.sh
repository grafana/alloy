#!/bin/sh
set -e

# Default path where Alloy expects its config
CONFIG_PATH="/etc/alloy/config.alloy"

# Check if the BASE64 variable is populated
if [ -n "$GRAFANA_ALLOY_BASE64_CONFIG" ]; then
    echo "Decoding GRAFANA_ALLOY_BASE64_CONFIG into $CONFIG_PATH..."
    
    # Ensure the parent directory exists
    mkdir -p "$(dirname "$CONFIG_PATH")"
    
    # Decode the base64 string into the configuration file
    echo "$GRAFANA_ALLOY_BASE64_CONFIG" | base64 -d > "$CONFIG_PATH"
fi

# Execute the original container command (e.g., /bin/alloy run ...)
exec "$@"
