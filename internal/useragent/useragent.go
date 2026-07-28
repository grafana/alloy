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
	// ExtensionProductName is the product name reported when Alloy runs as an OTel
	// Collector extension, distinguishing it from native Alloy.
	ExtensionProductName = ProductName + " OTel Extension"

	deployModeEnv = "ALLOY_DEPLOY_MODE"

	EngineOTel    = "otel"
	EngineDefault = "default"
)

// settable by tests
var goos = runtime.GOOS
var executable = os.Executable
var args = os.Args

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
	// The product name distinguishes the engine: native Alloy vs Alloy running as
	// an OTel Collector extension.
	product := ProductName
	if GetEngineMode() == EngineOTel {
		product = ExtensionProductName
	}
	return fmt.Sprintf("%s/%s%s", product, build.Version, parenthesis)
}

// GetEngineMode returns which engine Alloy is running with.
func GetEngineMode() string {
	// Find the first positional argument (the subcommand). Skip flags and any
	// flag values that may precede it.
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if a == EngineOTel {
			return EngineOTel
		}
		// The first positional argument is the subcommand; if it isn't "otel"
		// we are not running the OTel engine.
		break
	}
	return EngineDefault
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
