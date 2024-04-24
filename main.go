package main

import (
	"github.com/grafana/alloy/internal/alloycli"
	"github.com/grafana/alloy/internal/build"
	"github.com/prometheus/client_golang/prometheus"
	"runtime/debug"

	// Register Prometheus SD components
	_ "github.com/grafana/loki/clients/pkg/promtail/discovery/consulagent"
	_ "github.com/prometheus/prometheus/discovery/install"

	// Register integrations
	_ "github.com/grafana/alloy/internal/static/integrations/install"

	// Embed a set of fallback X.509 trusted roots
	// Allows the app to work correctly even when the OS does not provide a verifier or systems roots pool
	_ "golang.org/x/crypto/x509roots/fallback"
)

func init() {
	prometheus.MustRegister(build.NewCollector("alloy"))
}

func main() {
	// It is recommended increasing GOGC if go_memstats_gc_cpu_fraction exceeds 0.05 for extended periods of time.
	debug.SetGCPercent(30)
	alloycli.Run()
}
