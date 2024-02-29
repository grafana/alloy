// package useragent provides a consistent way to get a user agent for outbound http requests from Grafana Agent.
// The default User-Agent is `GrafanaAgent/$VERSION($MODE)`
// Where version is the build version of the agent and MODE is one of "static" or "flow".
package useragent

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/grafana/agent/internal/build"
)

const (
	deployModeEnv = "AGENT_DEPLOY_MODE"
	modeEnv       = "AGENT_MODE"
)

// settable by tests
var goos = runtime.GOOS
var executable = os.Executable

func Get() string {
	parenthesis := ""
	metadata := []string{}
	if mode := getRunMode(); mode != "" {
		metadata = append(metadata, mode)
	}
	metadata = append(metadata, goos)
	if op := GetDeployMode(); op != "" {
		metadata = append(metadata, op)
	}
	if len(metadata) > 0 {
		parenthesis = fmt.Sprintf(" (%s)", strings.Join(metadata, "; "))
	}
	return fmt.Sprintf("GrafanaAgent/%s%s", build.Version, parenthesis)
}

// getRunMode attempts to get agent mode, using `unknown` for invalid values.
func getRunMode() string {
	key := os.Getenv(modeEnv)
	switch key {
	case "flow":
		return "flow"
	case "static", "":
		return "static"
	default:
		return "unknown"
	}
}

// GetDeployMode returns our best-effort guess at the way Grafana Agent was deployed.
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
