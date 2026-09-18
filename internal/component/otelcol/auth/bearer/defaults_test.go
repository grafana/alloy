package bearer_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/auth/bearer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args bearer.Arguments
	args.SetToDefault()

	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	expected := &bearertokenauthextension.Config{
		Header: "Authorization",
		Scheme: "Bearer",
	}

	client, err := args.ConvertClient()
	require.NoError(t, err)
	require.Equal(t, expected, client)

	server, err := args.ConvertServer()
	require.NoError(t, err)
	require.Equal(t, expected, server)
}
