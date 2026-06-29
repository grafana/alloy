package otelcol_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/stretchr/testify/require"
)

func TestHTTPServerArguments_ConvertTimeoutDefaults(t *testing.T) {
	args := &otelcol.HTTPServerArguments{}
	cfg, err := args.Convert()
	require.NoError(t, err)

	server := cfg.Get()
	require.NotNil(t, server)
	require.Equal(t, 1*time.Minute, server.IdleTimeout)
	require.Equal(t, 1*time.Minute, server.ReadHeaderTimeout)
	require.Equal(t, 30*time.Second, server.WriteTimeout)
	require.Equal(t, time.Duration(0), server.ReadTimeout)
}

func TestHTTPServerArguments_ConvertTimeoutCustom(t *testing.T) {
	args := &otelcol.HTTPServerArguments{
		IdleTimeout:       2 * time.Minute,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      45 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
	}
	cfg, err := args.Convert()
	require.NoError(t, err)

	server := cfg.Get()
	require.NotNil(t, server)
	require.Equal(t, 2*time.Minute, server.IdleTimeout)
	require.Equal(t, 10*time.Second, server.ReadTimeout)
	require.Equal(t, 45*time.Second, server.WriteTimeout)
	require.Equal(t, 15*time.Second, server.ReadHeaderTimeout)
}
