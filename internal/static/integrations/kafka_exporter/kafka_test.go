package kafka_exporter

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/config"
)

func TestConfig_SecretKafkaPassword(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  kafka_exporter:
    enabled: true
    sasl_password: secret_password
`
	config.CheckSecret(t, stringCfg, "secret_password")
}

func TestNew_KafkaUnreachableAtStartup(t *testing.T) {
	// Reserve a local port and close it again so the dial is refused.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	cfg := DefaultConfig
	cfg.KafkaURIs = []string{addr}

	integration, err := New(slog.New(slog.DiscardHandler), &cfg)
	require.NoError(t, err, "an unreachable broker must not fail integration construction")

	handler, err := integration.MetricsHandler()
	require.NoError(t, err)

	body := scrape(t, handler)
	require.Contains(t, body, "kafka_up 0")
	require.NotContains(t, body, "kafka_brokers")

	// Once a broker becomes reachable the next scrape connects and exports metrics.
	broker := sarama.NewMockBrokerAddr(t, 1, addr)
	defer broker.Close()
	broker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest":       sarama.NewMockMetadataResponse(t).SetBroker(addr, broker.BrokerID()).SetController(broker.BrokerID()),
		"ListGroupsRequest":     sarama.NewMockListGroupsResponse(t),
		"DescribeGroupsRequest": sarama.NewMockDescribeGroupsResponse(t),
	})

	body = scrape(t, handler)
	require.Contains(t, body, "kafka_up 1")
	require.Contains(t, body, "kafka_brokers 1")
}

func TestNew_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(c *Config)
		err    string
	}{
		{
			name:   "empty kafka_uris",
			mutate: func(c *Config) { c.KafkaURIs = nil },
			err:    "empty kafka_uris provided",
		},
		{
			name:   "sasl without credentials",
			mutate: func(c *Config) { c.UseSASL = true },
			err:    "SASL is enabled but username or password was not provided",
		},
		{
			name:   "zookeeper lag without uris",
			mutate: func(c *Config) { c.UseZooKeeperLag = true },
			err:    "zookeeper lag is enabled but no zookeeper uri was provided",
		},
		{
			name:   "invalid kafka_version",
			mutate: func(c *Config) { c.KafkaVersion = "not-a-version" },
			err:    "invalid kafka_version",
		},
		{
			name:   "invalid metadata_refresh_interval",
			mutate: func(c *Config) { c.MetadataRefreshInterval = "soon" },
			err:    "invalid metadata_refresh_interval",
		},
		{
			name:   "invalid topics_filter_regex",
			mutate: func(c *Config) { c.TopicsFilter = "(" },
			err:    "invalid topics_filter_regex",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig
			cfg.KafkaURIs = []string{"127.0.0.1:9092"}
			tc.mutate(&cfg)

			_, err := New(slog.New(slog.DiscardHandler), &cfg)
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func scrape(t *testing.T, handler http.Handler) string {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}
