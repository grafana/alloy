#!/usr/bin/env sh
# Wrapper to run Alloy's embedded OpAMP supervisor as PID 1.
# Delegate to Alloy's otel-supervisor subcommand and pass through all arguments.
# Using exec makes the supervisor PID 1 so it receives SIGTERM directly
exec /bin/alloy otel-supervisor "$@"
