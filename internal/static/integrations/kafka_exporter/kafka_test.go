package kafka_exporter //nolint:golint

import (
	"testing"

	"github.com/grafana/agent/internal/static/config"
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
