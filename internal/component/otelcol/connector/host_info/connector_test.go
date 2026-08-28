package host_info

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestNewConnector(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		hostIdentifiers      []string
		metricsFlushInterval *time.Duration
		expectedConfig       *Config
	}{
		{
			name:           "default config",
			expectedConfig: createDefaultConfig().(*Config),
		},
		{
			name:                 "other config",
			hostIdentifiers:      []string{"host.id", "host.name", "k8s.node.uid"},
			metricsFlushInterval: durationPtr(15 * time.Second),
			expectedConfig: &Config{
				HostIdentifiers:      []string{"host.id", "host.name", "k8s.node.uid"},
				MetricsFlushInterval: 15 * time.Second,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			if tc.hostIdentifiers != nil {
				cfg.HostIdentifiers = tc.hostIdentifiers
			}
			if tc.metricsFlushInterval != nil {
				cfg.MetricsFlushInterval = *tc.metricsFlushInterval
			}

			c, err := factory.CreateTracesToMetrics(t.Context(), connectortest.NewNopSettings(component.MustNewType("hostinfoconnector")), cfg, consumertest.NewNop())
			imp := c.(*connectorImp)

			assert.NoError(t, err)
			assert.NotNil(t, imp)
			assert.Equal(t, tc.expectedConfig.HostIdentifiers, imp.config.HostIdentifiers)
			assert.Equal(t, tc.expectedConfig.MetricsFlushInterval, imp.config.MetricsFlushInterval)
		})
	}
}

func durationPtr(t time.Duration) *time.Duration {
	return &t
}
