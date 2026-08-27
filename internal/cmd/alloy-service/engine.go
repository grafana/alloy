package main

// isOtelMode reports whether value (an ALLOY_OTEL_MODE-style toggle)
// selects the OTel engine. Mirrors the truthy check in
// packaging/systemd/alloy-wrapper and the Homebrew wrapper: "1", "true",
// "yes", "on" (case-sensitive) select the OTel engine ("alloy otel");
// anything else, including "" (unset or absent), keeps the default engine
// ("alloy run").
func isOtelMode(value string) bool {
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// resolveEngineArgs computes the Alloy binary argv (excluding the binary
// path itself) to launch, given the ALLOY_OTEL_MODE registry value and the
// Arguments the installer wrote for the default engine.
//
//   - otelMode is the raw ALLOY_OTEL_MODE registry value.
//   - otelConfigDefault is the OTel engine's config path. The OTel engine
//     always reads its config from a fixed, installer-provisioned
//     location (a sibling of the default engine's own installed config,
//     inside the install directory) — it does not follow a custom
//     install-time config path. The caller builds this path before
//     calling in, since path joining is platform-specific and this
//     function must stay OS-agnostic.
//   - defaultEngineArgs is Args exactly as read from the registry
//     Arguments value (e.g. ["run", "<config>", "--storage.path=...", ...]).
//     It is returned unmodified whenever otelMode is not truthy, so
//     existing installs are byte-for-byte unaffected.
func resolveEngineArgs(otelMode, otelConfigDefault string, defaultEngineArgs []string) []string {
	if !isOtelMode(otelMode) {
		return defaultEngineArgs
	}
	return []string{"otel", "--config=" + otelConfigDefault}
}
