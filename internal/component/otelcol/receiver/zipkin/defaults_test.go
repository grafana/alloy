package zipkin_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/zipkin"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
)

func TestDefaultArguments(t *testing.T) {
	var args zipkin.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &zipkinreceiver.Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  "0.0.0.0:9411",
				Transport: confignet.TransportTypeTCP,
			},
			CompressionAlgorithms: []string{"", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"},
			ReadHeaderTimeout:     time.Minute,
			WriteTimeout:          30 * time.Second,
			IdleTimeout:           time.Minute,
			KeepAlivesEnabled:     true,
		},
		ParseStringTags: false,
	}, cfg)
}
