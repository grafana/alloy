package host_info

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

func TestConsumeTraces(t *testing.T) {
	for _, tc := range []struct {
		name            string
		hostIdentifiers []string
		resources       []map[string]any
		scopeSpans      int
		expectedHosts   []string
	}{
		{
			name:            "first matching identifier wins",
			hostIdentifiers: []string{"k8s.node.name", "host.id"},
			resources:       []map[string]any{{"k8s.node.name": "node-1", "host.id": "i-abc"}},
			scopeSpans:      1,
			expectedHosts:   []string{"node-1"},
		},
		{
			name:            "later identifier used when the earlier one is absent",
			hostIdentifiers: []string{"k8s.node.name", "host.id"},
			resources:       []map[string]any{{"host.id": "i-abc"}},
			scopeSpans:      1,
			expectedHosts:   []string{"i-abc"},
		},
		{
			// Beyla reports both attributes for a node, a collector using the
			// k8s attributes processor reports only k8s.node.name.
			name:            "one host across sources reporting different attributes",
			hostIdentifiers: []string{"k8s.node.name", "host.id"},
			resources: []map[string]any{
				{"k8s.node.name": "node-1", "host.id": "i-abc"},
				{"k8s.node.name": "node-1"},
			},
			scopeSpans:    1,
			expectedHosts: []string{"node-1"},
		},
		{
			name:            "resource without scope spans",
			hostIdentifiers: []string{"k8s.node.name"},
			resources:       []map[string]any{{"k8s.node.name": "node-1"}},
			scopeSpans:      0,
			expectedHosts:   []string{"node-1"},
		},
		{
			name:            "non-string identifier value",
			hostIdentifiers: []string{"host.id"},
			resources:       []map[string]any{{"host.id": int64(42)}},
			scopeSpans:      1,
			expectedHosts:   nil,
		},
		{
			name:            "no identifier present",
			hostIdentifiers: []string{"host.id"},
			resources:       []map[string]any{{"service.name": "foo"}},
			scopeSpans:      1,
			expectedHosts:   nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &connectorImp{
				config:      Config{HostIdentifiers: tc.hostIdentifiers},
				hostMetrics: newHostMetrics(),
			}

			require.NoError(t, c.ConsumeTraces(t.Context(), testTraces(tc.resources, tc.scopeSpans)))

			metrics, count := c.hostMetrics.metrics()
			require.Equal(t, len(tc.expectedHosts), count)
			if len(tc.expectedHosts) == 0 {
				return
			}

			dps := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
			var hosts []string
			for i := 0; i < dps.Len(); i++ {
				val, ok := dps.At(i).Attributes().Get(hostIdentifierAttr)
				require.True(t, ok)
				hosts = append(hosts, val.AsString())
			}
			assert.ElementsMatch(t, tc.expectedHosts, hosts)
		})
	}
}

func testTraces(resources []map[string]any, scopeSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	for _, attrs := range resources {
		rs := td.ResourceSpans().AppendEmpty()
		for k, v := range attrs {
			switch val := v.(type) {
			case string:
				rs.Resource().Attributes().PutStr(k, val)
			case int64:
				rs.Resource().Attributes().PutInt(k, val)
			}
		}
		for i := 0; i < scopeSpans; i++ {
			rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		}
	}
	return td
}

func durationPtr(t time.Duration) *time.Duration {
	return &t
}
