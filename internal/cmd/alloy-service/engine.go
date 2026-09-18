package main

import "strings"

func isOtelMode(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func resolveEngineArgs(otelMode, otelConfigDefault string, otelArguments, defaultEngineArgs []string) []string {
	if !isOtelMode(otelMode) {
		return defaultEngineArgs
	}
	// otelArguments is appended after --config= because otelcol merges
	// repeated --config flags in argv order, so a --config the user put in
	// otelArguments overrides this installer default.
	return append([]string{"otel", "--config=" + otelConfigDefault}, otelArguments...)
}
