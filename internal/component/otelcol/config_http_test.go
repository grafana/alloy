package otelcol_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/stretchr/testify/require"
)

// Convert passes timeouts through unchanged; defaults are applied by each
// component's SetToDefault, not here. That lets a component explicitly set
// a timeout to 0 (unbounded), which would otherwise be indistinguishable
// from "not set".
func TestHTTPServerArguments_ConvertTimeoutZeroValue(t *testing.T) {
	args := &otelcol.HTTPServerArguments{}
	cfg, err := args.Convert()
	require.NoError(t, err)

	server := cfg.Get()
	require.NotNil(t, server)
	//nolint:staticcheck // IdleTimeout remains the source of truth for an already-unmarshaled ServerConfig.
	require.Equal(t, time.Duration(0), server.IdleTimeout)
	require.Equal(t, time.Duration(0), server.ReadHeaderTimeout)
	require.Equal(t, time.Duration(0), server.WriteTimeout)
	require.Equal(t, time.Duration(0), server.ReadTimeout)
}

func TestCORSArguments_ConvertExposedHeaders(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		cors := (&otelcol.CORSArguments{}).Convert()
		require.Nil(t, cors.Get().ExposedHeaders)
	})

	t.Run("set", func(t *testing.T) {
		args := &otelcol.CORSArguments{
			ExposedHeaders: []string{"X-Request-Id", "X-Trace-Id"},
		}
		cors := args.Convert()
		require.Equal(t, []string{"X-Request-Id", "X-Trace-Id"}, cors.Get().ExposedHeaders)
	})
}

func TestHTTPClientArguments_ConvertKeepalive(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		args := &otelcol.HTTPClientArguments{}
		cfg, err := args.Convert()
		require.NoError(t, err)
		require.False(t, cfg.Keepalive.HasValue())
	})

	t.Run("set", func(t *testing.T) {
		args := &otelcol.HTTPClientArguments{
			Keepalive: &otelcol.KeepaliveArguments{
				IdleConnTimeout:     30 * time.Second,
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 10,
			},
		}
		cfg, err := args.Convert()
		require.NoError(t, err)
		require.True(t, cfg.Keepalive.HasValue())
		ka := cfg.Keepalive.Get()
		require.Equal(t, 30*time.Second, ka.IdleConnTimeout)
		require.Equal(t, 50, ka.MaxIdleConns)
		require.Equal(t, 10, ka.MaxIdleConnsPerHost)
	})
}

func TestKeepaliveArguments_SetToDefault(t *testing.T) {
	var args otelcol.KeepaliveArguments
	args.SetToDefault()
	require.Equal(t, otelcol.DefaultKeepaliveIdleConnTimeout, args.IdleConnTimeout)
	require.Equal(t, otelcol.DefaultKeepaliveMaxIdleConns, args.MaxIdleConns)
	require.Equal(t, 0, args.MaxIdleConnsPerHost)
}

func TestHTTPServerArguments_ConvertKeepalive(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		args := &otelcol.HTTPServerArguments{}
		cfg, err := args.Convert()
		require.NoError(t, err)
		require.False(t, cfg.Get().Keepalive.HasValue())
	})

	t.Run("set", func(t *testing.T) {
		args := &otelcol.HTTPServerArguments{
			Keepalive: &otelcol.HTTPKeepaliveServerArguments{
				IdleTimeout: 30 * time.Second,
			},
		}
		cfg, err := args.Convert()
		require.NoError(t, err)
		require.True(t, cfg.Get().Keepalive.HasValue())
		require.Equal(t, 30*time.Second, cfg.Get().Keepalive.Get().IdleTimeout)
	})
}

func TestHTTPKeepaliveServerArguments_SetToDefault(t *testing.T) {
	var args otelcol.HTTPKeepaliveServerArguments
	args.SetToDefault()
	require.Equal(t, otelcol.DefaultKeepaliveServerIdleTimeout, args.IdleTimeout)
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
	//nolint:staticcheck // IdleTimeout remains the source of truth for an already-unmarshaled ServerConfig.
	require.Equal(t, 2*time.Minute, server.IdleTimeout)
	require.Equal(t, 10*time.Second, server.ReadTimeout)
	require.Equal(t, 45*time.Second, server.WriteTimeout)
	require.Equal(t, 15*time.Second, server.ReadHeaderTimeout)
}
