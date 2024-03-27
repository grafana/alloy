// package useragent provides a consistent way to get a user agent for outbound
// http requests from Grafana Alloy. The default User-Agent is `Alloy/VERSION
// (METADATA)`, where VERSION is the build version of Alloy and METADATA
// includes information about how Alloy was deployed.
package useragent

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/grafana/alloy/internal/build"
)

const (
	ProductName = "Alloy"

	deployModeEnv = "ALLOY_DEPLOY_MODE"
)

// settable by tests
var goos = runtime.GOOS
var executable = os.Executable

func Get() string {
	parenthesis := ""
	metadata := []string{}
	metadata = append(metadata, goos)
	if op := GetDeployMode(); op != "" {
		metadata = append(metadata, op)
	}
	if len(metadata) > 0 {
		parenthesis = fmt.Sprintf(" (%s)", strings.Join(metadata, "; "))
	}
	return fmt.Sprintf("%s/%s%s", ProductName, build.Version, parenthesis)
}

// GetDeployMode returns our best-effort guess at the way Grafana Alloy was deployed.
func GetDeployMode() string {
	op := os.Getenv(deployModeEnv)
	// only return known modes. Use "binary" as a default catch-all.
	switch op {
	case "operator", "helm", "docker", "deb", "rpm", "brew":
		return op
	}
	// try to detect if executable is in homebrew directory
	if path, err := executable(); err == nil && goos == "darwin" && strings.Contains(path, "brew") {
		return "brew"
	}
	// fallback to binary
	return "binary"
}
