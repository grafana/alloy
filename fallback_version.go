package main

import (
	_ "embed"
	"encoding/json"

	"github.com/grafana/alloy/internal/build"
)

//go:embed .release-please-manifest.json
var fallbackVersionJSON []byte

// fallbackVersion returns a version string to use for when the version isn't
// explicitly set at build time. The version string will always have -devel
// appended to it.
func fallbackVersion() string {
	return fallbackVersionFromJSON(fallbackVersionJSON)
}

func fallbackVersionFromJSON(data []byte) string {
	var manifest map[string]string
	if err := json.Unmarshal(data, &manifest); err != nil {
		// We shouldn't hit this case since we always control the contents of the
		// manifest file, but just in case we'll return the existing version.
		return build.Version
	}

	version, ok := manifest["."]
	if !ok || version == "" {
		return build.Version
	}

	// The manifest stores versions without the "v" prefix, so add it
	return "v" + version + "-devel"
}
