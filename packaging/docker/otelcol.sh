#!/usr/bin/env sh
# Compatibility wrapper for OpenTelemetry Collector integrations.
# Delegate to Alloy's OTel engine and pass through all arguments.
exec /bin/alloy otel "$@"
