package integrations

import (
	"context"
	"net/http"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/static/integrations/config"
)

// Config provides the configuration and constructor for an integration.
type Config interface {
	// Name returns the name of the integration and the key that will be used to
	// pull the configuration from the Agent config YAML.
	Name() string

	// InstanceKey should return the key that represents the config, which will be
	// used to populate the value of the `instance` label for metrics.
	//
	// InstanceKey is given a defaultKey that can be used to represent this exporter if
	// the implementation cannot provide a better alternative based on config. What defaultKey
	// is set to, will depend on the scope of the exporter. For example, if the exporter collects
	// metrics from the entire machine, it may be the hostname of the local machine.
	//
	// This method may not be invoked if the instance key for a Config is
	// overridden.
	InstanceKey(defaultKey string) (string, error)

	// NewIntegration returns an integration for the given with the given logger.
	NewIntegration(l log.Logger) (Integration, error)
}

// An Integration is a process that integrates with some external system and
// pulls telemetry data.
type Integration interface {
	// MetricsHandler returns an http.Handler that will return metrics.
	MetricsHandler() (http.Handler, error)

	// ScrapeConfigs returns a set of scrape configs that determine where metrics
	// can be scraped.
	ScrapeConfigs() []config.ScrapeConfig

	// Run should start the integration and do any required tasks, if necessary.
	// For example, an Integration that requires a persistent connection to a
	// database would establish that connection here. If the integration doesn't
	// need to do anything, it should wait for the ctx to be canceled.
	//
	// An error will be returned if the integration failed. Integrations should
	// not return the ctx error.
	Run(ctx context.Context) error
}

// NewIntegrationWithInstanceKey uses cfg to construct an integration and
// return it along its instance key.
func NewIntegrationWithInstanceKey(l log.Logger, cfg Config, defaultKey string) (Integration, string, error) {
	key, err := cfg.InstanceKey(defaultKey)
	if err != nil {
		return nil, key, err
	}
	integration, err := cfg.NewIntegration(l)
	return integration, key, err
}
