package flowcmd

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/alloycli"
	"github.com/grafana/alloy/internal/build"

	// Register Prometheus SD components
	_ "github.com/grafana/alloy/internal/loki/promtail/discovery/consulagent"
	_ "github.com/prometheus/prometheus/discovery/install"

	// Register integrations
	_ "github.com/grafana/alloy/internal/static/integrations/install"

	// Embed a set of fallback X.509 trusted roots
	// Allows the app to work correctly even when the OS does not provide a verifier or systems roots pool
	_ "golang.org/x/crypto/x509roots/fallback"

	// Embed application manifest for Windows builds
	_ "github.com/grafana/alloy/internal/winmanifest"
)

func init() {
	// If the build version wasn't set by the build process, we'll set it based
	// on the version string in VERSION.
	if build.Version == "" || build.Version == "v0.0.0" {
		build.Version = fallbackVersion()
	}

	prometheus.MustRegister(build.NewCollector("alloy"))
}

// RootCommand exposes the root Cobra command constructed by the internal alloy CLI.
func RootCommand() *cobra.Command {
	return alloycli.Command()
}

func RunCommand() *cobra.Command {
	return alloycli.RunCommand()
}
