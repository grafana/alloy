// Package snmp_exporter embeds https://github.com/prometheus/snmp_exporter
package snmp_exporter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/config"
	snmp_common "github.com/grafana/alloy/internal/static/integrations/snmp_exporter/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/snmp_exporter/collector"
	snmp_config "github.com/prometheus/snmp_exporter/config"
)

// DefaultConfig holds the default settings for the snmp_exporter integration.
var DefaultConfig = Config{
	WalkParams:     make(map[string]snmp_config.WalkParams),
	SnmpConfigFile: "",
	SnmpTargets:    make([]SNMPTarget, 0),
	SnmpConfig:     snmp_config.Config{},
}

// SNMPTarget defines a target device to be used by the integration.
type SNMPTarget struct {
	Name        string `yaml:"name"`
	Target      string `yaml:"address"`
	Module      string `yaml:"module"`
	Auth        string `yaml:"auth"`
	WalkParams  string `yaml:"walk_params,omitempty"`
	SNMPContext string `yaml:"snmp_context,omitempty"`
}

// Config configures the SNMP integration.
type Config struct {
	WalkParams     map[string]snmp_config.WalkParams `yaml:"walk_params,omitempty"`
	SnmpConfigFile string                            `yaml:"config_file,omitempty"`
	SnmpTargets    []SNMPTarget                      `yaml:"snmp_targets"`
	SnmpConfig     snmp_config.Config                `yaml:"snmp_config,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration.
func (c *Config) Name() string {
	return "snmp"
}

// InstanceKey returns the hostname:port of the agent.
func (c *Config) InstanceKey(agentKey string) (string, error) {
	return agentKey, nil
}

// NewIntegration creates a new SNMP integration.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	return New(l, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
}

// New creates a new snmp_exporter integration
func New(log log.Logger, c *Config) (integrations.Integration, error) {
	snmpCfg, err := LoadSNMPConfig(c.SnmpConfigFile, &c.SnmpConfig)
	if err != nil {
		return nil, err
	}
	// The `name` and `address` fields are mandatory for the SNMP targets are mandatory.
	// Enforce this check and fail the creation of the integration if they're missing.
	for _, target := range c.SnmpTargets {
		if target.Name == "" || target.Target == "" {
			return nil, fmt.Errorf("failed to load snmp_targets; the `name` and `address` fields are mandatory")
		}
	}

	sh := &snmpHandler{
		cfg:     c,
		snmpCfg: snmpCfg,
		log:     log,
	}
	integration := &Integration{
		sh: sh,
	}

	return integration, nil
}

// LoadSNMPConfig loads the SNMP configuration from the given file. If the file is empty, it will
// load the embedded configuration.
func LoadSNMPConfig(snmpConfigFile string, snmpCfg *snmp_config.Config) (*snmp_config.Config, error) {
	var err error
	if snmpConfigFile != "" {
		snmpCfg, err = snmp_config.LoadFile([]string{snmpConfigFile}, false)
		if err != nil {
			return nil, fmt.Errorf("failed to load snmp config from file %v: %w", snmpConfigFile, err)
		}
	} else {
		if len(snmpCfg.Modules) == 0 && len(snmpCfg.Auths) == 0 { // If the user didn't specify a config, load the embedded config.
			snmpCfg, err = snmp_common.LoadEmbeddedConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to load embedded snmp config: %w", err)
			}
		}
	}
	return snmpCfg, nil
}

func NewSNMPMetrics(reg prometheus.Registerer) collector.Metrics {
	buckets := prometheus.ExponentialBuckets(0.0001, 2, 15)
	return collector.Metrics{
		SNMPCollectionDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "collection_duration_seconds",
				Help:      "Duration of collections by the SNMP exporter",
			},
			[]string{"module"},
		),
		SNMPUnexpectedPduType: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "unexpected_pdu_type_total",
				Help:      "Unexpected Go types in a PDU.",
			},
		),
		SNMPDuration: promauto.With(reg).NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "packet_duration_seconds",
				Help:      "A histogram of latencies for SNMP packets.",
				Buckets:   buckets,
			},
		),
		SNMPPackets: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "packets_total",
				Help:      "Number of SNMP packet sent, including retries.",
			},
		),
		SNMPRetries: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "packet_retries_total",
				Help:      "Number of SNMP packet retries.",
			},
		),
		SNMPInflight: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "request_in_flight",
				Help:      "Current number of SNMP scrapes being requested.",
			},
		),
	}
}

// Integration is the SNMP integration. The integration scrapes metrics
// from the host Linux-based system.
type Integration struct {
	sh *snmpHandler
}

// MetricsHandler implements Integration.
func (i *Integration) MetricsHandler() (http.Handler, error) {
	return i.sh, nil
}

// Run satisfies Integration.Run.
func (i *Integration) Run(ctx context.Context) error {
	// We don't need to do anything here, so we can just wait for the context to
	// finish.
	<-ctx.Done()
	return ctx.Err()
}

// ScrapeConfigs satisfies Integration.ScrapeConfigs.
func (i *Integration) ScrapeConfigs() []config.ScrapeConfig {
	var res []config.ScrapeConfig
	for _, target := range i.sh.cfg.SnmpTargets {
		queryParams := url.Values{}
		queryParams.Add("target", target.Target)
		if target.Module != "" {
			queryParams.Add("module", target.Module)
		}
		if target.Auth != "" {
			queryParams.Add("auth", target.Auth)
		}
		if target.WalkParams != "" {
			queryParams.Add("walk_params", target.WalkParams)
		}
		if target.SNMPContext != "" {
			queryParams.Add("snmp_context", target.SNMPContext)
		}
		res = append(res, config.ScrapeConfig{
			JobName:     i.sh.cfg.Name() + "/" + target.Name,
			MetricsPath: "/metrics",
			QueryParams: queryParams,
		})
	}
	return res
}
