package oci

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	ociconfig "github.com/grafana/oci-exporter/pkg/config"
)

// DefaultArguments holds default values for Arguments when unmarshaled from Alloy.
var DefaultArguments = Arguments{
	Debug:             false,
	ScrapeInterval:    5 * time.Minute,
	DiscoveryInterval: 60 * time.Minute,
	ScrapeDelay:       3 * time.Minute,
}

// Arguments configures the otelcol.receiver.oci component.
type Arguments struct {
	Debug             bool           `alloy:"debug,attr,optional"`
	Jobs              []JobArguments `alloy:"job,block"`
	ScrapeInterval    time.Duration  `alloy:"scrape_interval,attr,optional"`
	DiscoveryInterval time.Duration  `alloy:"discovery_interval,attr,optional"`
	ScrapeDelay       time.Duration  `alloy:"scrape_delay,attr,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// JobArguments configures a single OCI scrape job.
type JobArguments struct {
	Name         string                 `alloy:"name,attr"`
	TenancyOCID  string                 `alloy:"tenancy_ocid,attr"`
	Auth         AuthArguments          `alloy:"auth,block,optional"`
	Regions      []string               `alloy:"regions,attr"`
	Compartments []CompartmentArguments `alloy:"compartment,block"`
}

// AuthArguments configures OCI authentication.
type AuthArguments struct {
	Type           string `alloy:"type,attr,optional"`
	ConfigFilePath string `alloy:"config_file_path,attr,optional"`
	Profile        string `alloy:"profile,attr,optional"`
}

// CompartmentArguments configures a compartment to scrape.
type CompartmentArguments struct {
	ID string `alloy:"id,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
	a.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if len(a.Jobs) == 0 {
		return fmt.Errorf("at least one job block is required")
	}
	for i, job := range a.Jobs {
		if job.TenancyOCID == "" {
			return fmt.Errorf("job[%d]: tenancy_ocid must not be empty", i)
		}
		if len(job.Regions) == 0 {
			return fmt.Errorf("job[%d]: at least one region is required", i)
		}
		if len(job.Compartments) == 0 {
			return fmt.Errorf("job[%d]: at least one compartment block is required", i)
		}
	}
	return nil
}

// Convert maps Alloy Arguments to the oci-exporter library config.
func (a *Arguments) Convert() ociconfig.ExporterConfig {
	cfg := ociconfig.ExporterConfig{
		Defaults: ociconfig.DefaultsConfig{
			ScrapeInterval:    a.ScrapeInterval,
			DiscoveryInterval: a.DiscoveryInterval,
			ScrapeDelay:       a.ScrapeDelay,
		},
	}

	for _, job := range a.Jobs {
		authType := ociconfig.AuthTypeAPIKey
		if job.Auth.Type != "" {
			authType = ociconfig.AuthType(job.Auth.Type)
		}

		var compartments []ociconfig.CompartmentConfig
		for _, comp := range job.Compartments {
			compartments = append(compartments, ociconfig.CompartmentConfig{
				CompartmentID: comp.ID,
				DiscoveryMode: ociconfig.DiscoveryModeAuto,
			})
		}

		cfg.Jobs = append(cfg.Jobs, ociconfig.JobConfig{
			Name:        job.Name,
			TenancyOCID: job.TenancyOCID,
			Auth: ociconfig.AuthConfig{
				Type:           authType,
				ConfigFilePath: job.Auth.ConfigFilePath,
				Profile:        job.Auth.Profile,
			},
			Regions:      job.Regions,
			Compartments: compartments,
		})
	}

	cfg.SetDefaults()
	return cfg
}
