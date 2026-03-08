package oci

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/oci_exporter"
	"github.com/grafana/alloy/syntax"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.oci",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "oci"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	cfg := a.Convert()
	integrationCfg := oci_exporter.Config{
		ExporterConfig: cfg,
	}

	integration, err := oci_exporter.New(opts.Logger, &integrationCfg, a.Debug)
	if err != nil {
		return nil, "", err
	}

	return integration, getHash(a), nil
}

// getHash calculates the MD5 hash of the Alloy representation of the config.
func getHash(a Arguments) string {
	bytes, err := syntax.Marshal(a)
	if err != nil {
		return "<unknown>"
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:])
}
