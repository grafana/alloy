package mongodb_exporter //nolint:golint

import (
	"testing"

	"github.com/grafana/agent/internal/static/config"
)

func TestConfig_SecretMongoDB(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  mongodb_exporter:
    enabled: true
    mongodb_uri: secret_password_in_uri
`
	config.CheckSecret(t, stringCfg, "secret_password_in_uri")
}
