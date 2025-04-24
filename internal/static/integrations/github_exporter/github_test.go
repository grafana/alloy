package github_exporter

import (
	"testing"

	"github.com/grafana/alloy/internal/static/config"
	// register github_exporter
)

func TestConfig_SecretGithub(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  github_exporter:
    enabled: true
    api_token: secret_api`
	config.CheckSecret(t, stringCfg, "secret_api")
}
