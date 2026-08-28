package stdlib

import (
	"os"
	"strings"

	"runtime"

	"github.com/blang/semver/v4"
)

var Version string

var constants = map[string]string{
	"hostname": "", // Initialized via init function
	"os":       runtime.GOOS,
	"arch":     runtime.GOARCH,
	"version":  Version,
}

func init() {
	hostname, err := os.Hostname()
	if err == nil {
		constants["hostname"] = hostname
	}
	constants["version"] = normalizeVersion(Version)
}

// normalizeVersion normalizes the version string to always contain a "v"
// prefix. If version cannot be parsed as a semantic version, version is returned unmodified.
//
// if version is empty, normalizeVersion returns "v0.0.0".
func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "v0.0.0"
	}

	parsed, err := semver.ParseTolerant(version)
	if err != nil {
		return version
	}
	return "v" + parsed.String()
}
