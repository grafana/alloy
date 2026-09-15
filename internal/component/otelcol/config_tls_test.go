package otelcol_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestConvert_Settings(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		args := &otelcol.TLSSetting{
			CA:                          "ca-pem",
			CAFile:                      "ca-file",
			Cert:                        "cert-pem",
			CertFile:                    "cert-file",
			Key:                         alloytypes.Secret("key-pem"),
			KeyFile:                     "key-file",
			MinVersion:                  "1.2",
			MaxVersion:                  "1.3",
			ReloadInterval:              10 * time.Second,
			CipherSuites:                []string{"TLS_RSA_WITH_AES_128_CBC_SHA"},
			IncludeInsecureCipherSuites: true,
			IncludeSystemCACertsPool:    true,
			CurvePreferences:            []string{"X25519"},
			TPMConfig: &otelcol.TPMConfig{
				Enabled:   true,
				Path:      "/dev/tpmrm0",
				OwnerAuth: "owner-auth",
				Auth:      "auth",
			},
		}

		converted := args.Convert()

		require.Equal(t, &configtls.Config{
			CAPem:                       configopaque.String("ca-pem"),
			CAFile:                      "ca-file",
			CertPem:                     configopaque.String("cert-pem"),
			CertFile:                    "cert-file",
			KeyPem:                      configopaque.String("key-pem"),
			KeyFile:                     "key-file",
			MinVersion:                  "1.2",
			MaxVersion:                  "1.3",
			ReloadInterval:              10 * time.Second,
			CipherSuites:                []string{"TLS_RSA_WITH_AES_128_CBC_SHA"},
			IncludeInsecureCipherSuites: true,
			IncludeSystemCACertsPool:    true,
			CurvePreferences:            []string{"X25519"},
			TPMConfig: configtls.TPMConfig{
				Enabled:   true,
				Path:      "/dev/tpmrm0",
				OwnerAuth: "owner-auth",
				Auth:      "auth",
			},
		}, converted)
	})
}
