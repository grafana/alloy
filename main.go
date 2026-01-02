package main

import (
	"github.com/prometheus/client_golang/prometheus"

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
	// on the version in .release-please-manifest.json.
	if build.Version == "" || build.Version == "v0.0.0" {
		build.Version = fallbackVersion()
	}

	prometheus.MustRegister(build.NewCollector("alloy"))
}

func main() {
	alloycli.Run()
}
