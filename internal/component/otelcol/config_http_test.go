package otelcol_test

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
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

	t.Run("disable_keep_alives conflicts with keepalive", func(t *testing.T) {
		args := &otelcol.HTTPClientArguments{
			DisableKeepAlives: true,
			Keepalive:         &otelcol.KeepaliveArguments{},
		}
		_, err := args.Convert()
		require.Error(t, err)
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

	t.Run("keep_alives_enabled false conflicts with keepalive", func(t *testing.T) {
		disabled := false
		args := &otelcol.HTTPServerArguments{
			KeepAlivesEnabled: &disabled,
			Keepalive:         &otelcol.HTTPKeepaliveServerArguments{},
		}
		_, err := args.Convert()
		require.Error(t, err)
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

func TestHTTPClientArguments_Cookies(t *testing.T) {
	tests := []struct {
		name string
		cfg  string
		on   bool
	}{
		{name: "no block", cfg: `endpoint = "http://localhost"`, on: false},
		{name: "empty block", cfg: `
			endpoint = "http://localhost"
			cookies {}
		`, on: true},
		{name: "enabled true", cfg: `
			endpoint = "http://localhost"
			cookies { enabled = true }
		`, on: true},
		// enabled is a deprecated no-op, so false still turns cookies on.
		{name: "enabled false", cfg: `
			endpoint = "http://localhost"
			cookies { enabled = false }
		`, on: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args otelcol.HTTPClientArguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.cfg), &args))

			cfg, err := args.Convert()
			require.NoError(t, err)
			require.Equal(t, tc.on, cfg.Cookies.HasValue())
		})
	}
}

func TestHTTPClientArguments_LogDeprecations(t *testing.T) {
	for _, cfg := range []string{
		`endpoint = "http://localhost"
		cookies { enabled = true }`,
		`endpoint = "http://localhost"
		cookies { enabled = false }`,
	} {
		var args otelcol.HTTPClientArguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

		var buf bytes.Buffer
		args.LogDeprecations(slog.New(slog.NewTextHandler(&buf, nil)))
		require.Contains(t, buf.String(), "client.cookies.enabled is deprecated")
	}

	for _, cfg := range []string{
		`endpoint = "http://localhost"`,
		`endpoint = "http://localhost"
		cookies {}`,
	} {
		var args otelcol.HTTPClientArguments
		require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

		var buf bytes.Buffer
		args.LogDeprecations(slog.New(slog.NewTextHandler(&buf, nil)))
		require.Empty(t, buf.String())
	}

	require.NotPanics(t, func() {
		(&otelcol.HTTPClientArguments{}).LogDeprecations(nil)
	})
}
