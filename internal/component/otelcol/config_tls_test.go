package otelcol_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/stretchr/testify/require"
)

func TestTLSSetting_ConvertIncludeInsecureCipherSuites(t *testing.T) {
	t.Run("defaults to false", func(t *testing.T) {
		args := &otelcol.TLSSetting{}
		require.False(t, args.Convert().IncludeInsecureCipherSuites)
	})

	t.Run("passed through when set", func(t *testing.T) {
		args := &otelcol.TLSSetting{
			CipherSuites:                []string{"TLS_RSA_WITH_AES_128_CBC_SHA"},
			IncludeInsecureCipherSuites: true,
		}

		converted := args.Convert()
		require.True(t, converted.IncludeInsecureCipherSuites)
		require.Equal(t, []string{"TLS_RSA_WITH_AES_128_CBC_SHA"}, converted.CipherSuites)
	})
}
