package promsdprocessor

import (
	"context"
	"fmt"

	prom_config "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"gopkg.in/yaml.v2"
)

// TypeStr is the unique identifier for the Prometheus SD processor.
const TypeStr = "prom_sd_processor"

// Config holds the configuration for the Prometheus SD processor.
type Config struct {
	ScrapeConfigs   []interface{} `mapstructure:"scrape_configs"`
	OperationType   string        `mapstructure:"operation_type"`
	PodAssociations []string      `mapstructure:"pod_associations"`
}

// NewFactory returns a new factory for the Attributes processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		TypeStr,
		createDefaultConfig,
		processor.WithTraces(createTraceProcessor, component.StabilityLevelUndefined),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTraceProcessor(
	_ context.Context,
	cp processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {

	oCfg := cfg.(*Config)
	out, err := yaml.Marshal(oCfg.ScrapeConfigs)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal scrapeConfigs interface{} to yaml: %w", err)
	}

	scrapeConfigs := make([]*prom_config.ScrapeConfig, 0)
	err = yaml.Unmarshal(out, &scrapeConfigs)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal bytes to []*config.ScrapeConfig: %w", err)
	}

	return newTraceProcessor(nextConsumer, oCfg.OperationType, oCfg.PodAssociations, scrapeConfigs)
}
