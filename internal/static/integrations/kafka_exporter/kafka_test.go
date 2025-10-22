package kafka_exporter

import (
	"testing"

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
