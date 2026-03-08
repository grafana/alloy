// Package oci_exporter provides the OCI metrics exporter integration for Alloy.
package oci_exporter

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	ociconfig "github.com/grafana/oci-exporter/pkg/config"
	"github.com/grafana/oci-exporter/pkg/exporter"
	"github.com/grafana/oci-exporter/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.yaml.in/yaml/v2"
)

// Config controls the oci_exporter integration.
type Config struct {
	Debug          bool                     `yaml:"debug"`
	ExporterConfig ociconfig.ExporterConfig `yaml:"exporter_config"`
}

// DefaultConfig holds default options for the Config.
var DefaultConfig = Config{}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig
	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "oci_exporter"
}

// InstanceKey returns the key identifying this instance.
func (c *Config) InstanceKey(_ string) (string, error) {
	return getHash(c)
}

// getHash calculates the MD5 hash of the yaml representation of the config
func getHash(c *Config) (string, error) {
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// NewIntegration creates an OCI exporter integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c, c.Debug)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("oci"))
}

// ociIntegration implements integrations.Integration by wrapping the oci-exporter library.
type ociIntegration struct {
	name string
	exp  *exporter.Exporter
}

// New creates a new OCI exporter integration from the given config.
func New(logger log.Logger, c *Config, debug bool) (integrations.Integration, error) {
	c.ExporterConfig.SetDefaults()

	l := slog.New(newSlogHandler(logging.NewSlogGoKitHandler(logger), debug))

	exp, err := exporter.New(c.ExporterConfig, exporter.WithLogger(l))
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI exporter: %w", err)
	}

	name := "oci"
	if len(c.ExporterConfig.Jobs) > 0 {
		name = c.ExporterConfig.Jobs[0].Name
	}

	return &ociIntegration{
		name: name,
		exp:  exp,
	}, nil
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics
// by collecting OTel pmetric.Metrics from the exporter and converting them.
func (o *ociIntegration) MetricsHandler() (http.Handler, error) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		md, err := o.exp.Collect(r.Context())
		if err != nil {
			http.Error(w, fmt.Sprintf("collect failed: %v", err), http.StatusInternalServerError)
			return
		}

		reg := convertToPrometheus(md)
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
	return h, nil
}

// ScrapeConfigs returns scrape configs for the integration.
func (o *ociIntegration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     o.name,
		MetricsPath: "/metrics",
	}}
}

// Run doesn't need to do anything since the exporter is invoked on demand by the MetricsHandler.
// TODO(tristan): Consider adding a decoupled mode where the exporter runs in the background and MetricsHandler just serves from a local cache.
// Similar to how we set up the cloudwatch_exporter.
func (o *ociIntegration) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// convertToPrometheus converts OTel pmetric.Metrics to a Prometheus registry.
func convertToPrometheus(md pmetric.Metrics) *prometheus.Registry {
	reg := prometheus.NewRegistry()

	for i := range md.ResourceMetrics().Len() {
		rm := md.ResourceMetrics().At(i)
		for j := range rm.ScopeMetrics().Len() {
			sm := rm.ScopeMetrics().At(j)
			for k := range sm.Metrics().Len() {
				m := sm.Metrics().At(k)
				gauge := m.Gauge()
				if gauge.DataPoints().Len() == 0 {
					continue
				}

				// Collect all unique label names from data points.
				labelNames := collectLabelNames(gauge)

				gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
					Name: metrics.SanitizeMetricName(m.Name()),
					Help: m.Description(),
				}, labelNames)

				for l := range gauge.DataPoints().Len() {
					dp := gauge.DataPoints().At(l)
					labels := make(prometheus.Labels)
					for _, name := range labelNames {
						v, ok := dp.Attributes().Get(name)
						if ok {
							labels[name] = v.Str()
						} else {
							labels[name] = ""
						}
					}
					gv.With(labels).Set(dp.DoubleValue())
				}

				_ = reg.Register(gv)
			}
		}
	}

	return reg
}

// collectLabelNames returns a sorted list of unique attribute keys across all data points.
func collectLabelNames(gauge pmetric.Gauge) []string {
	seen := make(map[string]struct{})
	for i := range gauge.DataPoints().Len() {
		dp := gauge.DataPoints().At(i)
		dp.Attributes().Range(func(k string, _ pcommon.Value) bool {
			seen[k] = struct{}{}
			return true
		})
	}

	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	// Sort for deterministic output.
	sortStrings(names)
	return names
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
