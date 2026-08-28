package mysqld_exporter

import (
	"testing"

	"github.com/grafana/alloy/internal/static/config"
)

func TestConfig_SecretMysqlD(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  mysqld_exporter:
    enabled: true
    data_source_name: root:secret_password@myserver:3306`
	config.CheckSecret(t, stringCfg, "secret_password")
}
