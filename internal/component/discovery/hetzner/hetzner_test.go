package hetzner

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/hetzner"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/grafana/alloy/syntax"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyCfg := `
		port = 8080
		refresh_interval = "10m"
		role = "robot"
		http_headers = {
			"foo" = ["foobar"],
		}`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)

	assert.Equal(t, 8080, args.Port)
	assert.Equal(t, 10*time.Minute, args.RefreshInterval)
	assert.Equal(t, "robot", args.Role)

	header := args.HTTPClientConfig.HTTPHeaders.Headers["foo"][0]
	assert.Equal(t, "foobar", string(header))
}

func TestValidate(t *testing.T) {
	wrongRole := `
	role = "test"`

	var args Arguments
	err := syntax.Unmarshal([]byte(wrongRole), &args)
	require.ErrorContains(t, err, "unknown role test, must be one of robot or hcloud")
}

func TestConvert(t *testing.T) {
	args := Arguments{
		Role:            "robot",
		RefreshInterval: 60 * time.Second,
		Port:            80,
	}
	converted := args.Convert().(*prom_discovery.SDConfig)
	assert.Equal(t, 80, converted.Port)
	assert.Equal(t, model.Duration(60*time.Second), converted.RefreshInterval)
	assert.Equal(t, "robot", string(converted.Role))
}
