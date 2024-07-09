package build

import (
	"strings"

	"github.com/blang/semver/v4"
	"github.com/prometheus/client_golang/prometheus"
	cv "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/common/version"
)

// Version information passed to Prometheus version package. Package path as
// used by linker changes based on vendoring being used or not, so it's easier
// just to use stable Alloy path, and pass it to Prometheus in the code.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
)

func init() {
	Version = normalizeVersion(Version)
	injectVersion()
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

func injectVersion() {
	version.Version = Version
	version.Revision = Revision
	version.Branch = Branch
	version.BuildUser = BuildUser
	version.BuildDate = BuildDate
}

// NewCollector returns a collector that exports metrics about current
// version information.
func NewCollector(program string) prometheus.Collector {
	injectVersion()

	return cv.NewCollector(program)
}

// Print returns version information.
func Print(program string) string {
	injectVersion()

	return version.Print(program)
}
