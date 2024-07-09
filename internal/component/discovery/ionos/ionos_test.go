package ionos

import (
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/ionos"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyCfg := `
		datacenter_id = "datacenter_id"
		refresh_interval = "20s"
		port = 60
		basic_auth {
			username = "username"
			password = "pass"
		}
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	assert.Equal(t, "datacenter_id", args.DatacenterID)
	assert.Equal(t, 20*time.Second, args.RefreshInterval)
	assert.Equal(t, 60, args.Port)
	assert.Equal(t, "username", args.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, alloytypes.Secret("pass"), args.HTTPClientConfig.BasicAuth.Password)
}

func TestConvert(t *testing.T) {
	alloyArgs := Arguments{
		DatacenterID:    "datacenter_id",
		RefreshInterval: 20 * time.Second,
		Port:            81,
		HTTPClientConfig: config.HTTPClientConfig{
			BasicAuth: &config.BasicAuth{
				Username: "username",
				Password: "pass",
			},
		},
	}
	promArgs := alloyArgs.Convert().(*prom_discovery.SDConfig)
	assert.Equal(t, "datacenter_id", promArgs.DatacenterID)
	assert.Equal(t, model.Duration(20*time.Second), promArgs.RefreshInterval)
	assert.Equal(t, 81, promArgs.Port)
	assert.Equal(t, "username", promArgs.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("pass"), promArgs.HTTPClientConfig.BasicAuth.Password)
}

func TestValidateNoDatacenterId(t *testing.T) {
	alloyArgs := Arguments{
		RefreshInterval: 20 * time.Second,
		Port:            81,
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "datacenter_id can't be empty")
}

func TestValidateRefreshIntervalZero(t *testing.T) {
	alloyArgs := Arguments{
		DatacenterID:    "datacenter_id",
		RefreshInterval: 0,
		Port:            81,
	}
	err := alloyArgs.Validate()
	assert.Error(t, err, "refresh_interval must be greater than 0")
}
