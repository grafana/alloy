package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSWindowsCertificate(t *testing.T) {
	cfg := &TLSArguments{
		Cert: "asdf",
		WindowsFilter: &WindowsCertificateFilter{
			Server: &WindowsServerFilter{},
			Client: &WindowsClientFilter{},
		},
	}
	err := cfg.validateWindowsCertificateFilterTLS()
	require.Error(t, err)
	require.True(t, "cannot specify any key, certificate or CA when using windows certificate filter" == err.Error())
	cfg.Cert = ""
	cfg.CertFile = "asdf"
	err = cfg.validateWindowsCertificateFilterTLS()
	require.Error(t, err)
	require.True(t, "cannot specify any key, certificate or CA when using windows certificate filter" == err.Error())
	cfg.Cert = ""
	cfg.CertFile = ""
	err = cfg.validateWindowsCertificateFilterTLS()
	require.NoError(t, err)
}
