package main

import "strings"

// isOtelMode reports whether value (an ALLOY_OTEL_MODE-style toggle)
// selects the OTel engine. Mirrors the truthy check in
// packaging/systemd/alloy-wrapper and the Homebrew wrapper: "1", "true",
// "yes", "on" (case-insensitive) select the OTel engine ("alloy otel");
// anything else, including "" (unset or absent), keeps the default engine
// ("alloy run").
func isOtelMode(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// resolveEngineArgs computes the Alloy binary argv (excluding the binary
// path itself) to launch
func resolveEngineArgs(otelMode, otelConfigDefault string, otelArguments, defaultEngineArgs []string) []string {
	if !isOtelMode(otelMode) {
		return defaultEngineArgs
	}
	args := make([]string, 0, len(otelArguments)+2)
	args = append(args, "otel")
	args = append(args, "--config="+otelConfigDefault)
	args = append(args, otelArguments...)
	return args
}
