package fluentforward_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/fluentforward"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	alloyCfg := `
		endpoint = "localhost:1514"
		output {
		}
	`
	var args fluentforward.Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	require.NoError(t, args.Validate())

	comCfg, err := args.Convert()
	require.NoError(t, err)

	fluentComCfg, ok := comCfg.(*fluentforwardreceiver.Config)
	require.True(t, ok)

	assert.Equal(t, "localhost:1514", fluentComCfg.ListenAddress)
}

func TestConfigDefault(t *testing.T) {
	args := fluentforward.Arguments{}
	args.SetToDefault()

	fCfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults, which are empty today. If this fails, a contrib
	// bump added one: document it, then update these values.
	assert.Equal(t, &fluentforwardreceiver.Config{}, fCfg)

	assert.Error(t, args.Validate())
}
