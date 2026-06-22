package build

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/static/server"
)

func TestToServer_EmptyWindowsFilterMapsToZeroTLS(t *testing.T) {
	args := toServer(&server.Config{
		HTTP: server.HTTPConfig{
			TLSConfig: server.TLSConfig{
				WindowsCertificateFilter: &server.WindowsCertificateFilter{
					Server: &server.WindowsServerFilter{
						IssuerCommonNames: []string{},
					},
					Client: &server.WindowsClientFilter{
						IssuerCommonNames: []string{},
					},
				},
			},
		},
	})
	require.Nil(t, args.TLS.WindowsFilter)
	require.True(t, reflect.DeepEqual(*args.TLS, http.TLSArguments{}))
}

func TestToServer_NonEmptyWindowsFilterNotRemoved(t *testing.T) {
	args := toServer(&server.Config{
		HTTP: server.HTTPConfig{
			TLSConfig: server.TLSConfig{
				WindowsCertificateFilter: &server.WindowsCertificateFilter{
					Server: &server.WindowsServerFilter{
						Store: "my-store",
					},
				},
			},
		},
	})
	require.NotNil(t, args.TLS.WindowsFilter)
	require.NotNil(t, args.TLS.WindowsFilter.Server)
	require.Equal(t, "my-store", args.TLS.WindowsFilter.Server.Store)
	require.False(t, reflect.DeepEqual(*args.TLS, http.TLSArguments{}))
}
