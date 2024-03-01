package build

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

// Version information passed to Prometheus version package.
// Package path as used by linker changes based on vendoring being used or not,
// so it's easier just to use stable Agent path, and pass it to
// Prometheus in the code.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
)

func init() {
	injectVersion()
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

	return version.NewCollector(program)
}

// Print returns version information.
func Print(program string) string {
	injectVersion()

	return version.Print(program)
}
