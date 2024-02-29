package hetzner

import (
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRiverUnmarshal(t *testing.T) {
	riverCfg := `
		port = 8080
		refresh_interval = "10m"
		role = "robot"`

	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
	require.NoError(t, err)

	assert.Equal(t, 8080, args.Port)
	assert.Equal(t, 10*time.Minute, args.RefreshInterval)
	assert.Equal(t, "robot", args.Role)
}

func TestValidate(t *testing.T) {
	wrongRole := `
	role = "test"`

	var args Arguments
	err := river.Unmarshal([]byte(wrongRole), &args)
	require.ErrorContains(t, err, "unknown role test, must be one of robot or hcloud")
}

func TestConvert(t *testing.T) {
	args := Arguments{
		Role:            "robot",
		RefreshInterval: 60 * time.Second,
		Port:            80,
	}
	converted := args.Convert()
	assert.Equal(t, 80, converted.Port)
	assert.Equal(t, model.Duration(60*time.Second), converted.RefreshInterval)
	assert.Equal(t, "robot", string(converted.Role))
}
