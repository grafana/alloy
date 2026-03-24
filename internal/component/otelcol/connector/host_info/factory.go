package host_info

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr = "hostinfoconnector"
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HostIdentifiers:      []string{"host.id"},
		MetricsFlushInterval: 60 * time.Second,
	}
}

func createTracesToMetricsConnector(_ context.Context, params connector.Settings, cfg component.Config, next consumer.Metrics) (connector.Traces, error) {
	c := newConnector(params.Logger, cfg)
	c.metricsConsumer = next
	return c, nil
}
