package main

import (
	_ "embed"
	"strings"

	"github.com/grafana/alloy/internal/build"
)

//go:embed VERSION
var versionString string

// unsetBuildVersion is the value of build.Version when no version was
// injected via ldflags.
const unsetBuildVersion = "v0.0.0"

// CollectorVersion returns the version reported by the OTel Engine. It prefers
// the build version injected via ldflags, which carries pre-release and devel
// suffixes such as -rc.0, and falls back to the embedded VERSION file.
func CollectorVersion() string {
	return collectorVersion(build.Version, versionString)
}

func collectorVersion(buildVersion, fileContents string) string {
	if buildVersion != "" && buildVersion != unsetBuildVersion {
		return buildVersion
	}
	return CollectorVersionFromFile(fileContents)
}

func CollectorVersionFromFile(fileContents string) string {
	versionStringWithoutComment := strings.Split(fileContents, "#")[0]
	return "v" + strings.TrimSpace(versionStringWithoutComment)
}
