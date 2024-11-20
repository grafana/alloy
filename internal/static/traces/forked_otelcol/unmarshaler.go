// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package forked_otelcol // import "go.opentelemetry.io/collector/otelcol"

import (
	"github.com/grafana/alloy/internal/static/traces/forked_configunmarshaler"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/telemetry"
)

type ConfigSettings struct {
	Receivers  *forked_configunmarshaler.Configs[receiver.Factory]  `mapstructure:"receivers"`
	Processors *forked_configunmarshaler.Configs[processor.Factory] `mapstructure:"processors"`
	Exporters  *forked_configunmarshaler.Configs[exporter.Factory]  `mapstructure:"exporters"`
	Connectors *forked_configunmarshaler.Configs[connector.Factory] `mapstructure:"connectors"`
	Extensions *forked_configunmarshaler.Configs[extension.Factory] `mapstructure:"extensions"`
	Service    service.Config                                       `mapstructure:"service"`
}

// unmarshal the configSettings from a confmap.Conf.
// After the config is unmarshalled, `Validate()` must be called to validate.
func Unmarshal(v *confmap.Conf, factories otelcol.Factories) (*ConfigSettings, error) {

	telFactory := telemetry.NewFactory()
	defaultTelConfig := *telFactory.CreateDefaultConfig().(*telemetry.Config)

	// Unmarshal top level sections and validate.
	cfg := &ConfigSettings{
		Receivers:  forked_configunmarshaler.NewConfigs(factories.Receivers),
		Processors: forked_configunmarshaler.NewConfigs(factories.Processors),
		Exporters:  forked_configunmarshaler.NewConfigs(factories.Exporters),
		Connectors: forked_configunmarshaler.NewConfigs(factories.Connectors),
		Extensions: forked_configunmarshaler.NewConfigs(factories.Extensions),
		// TODO: Add a component.ServiceFactory to allow this to be defined by the Service.
		Service: service.Config{
			Telemetry: defaultTelConfig,
		},
	}

	return cfg, v.Unmarshal(&cfg)
}
