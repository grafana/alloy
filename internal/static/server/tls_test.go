package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"testing"

	kitlog "github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func Test_tlsListener(t *testing.T) {
	rawLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tlsConfig := TLSConfig{
		TLSCertPath: "testdata/example-cert.pem",
		TLSKeyPath:  "testdata/example-key.pem",
		ClientAuth:  "NoClientCert",
	}
	tlsLis, err := newTLSListener(rawLis, tlsConfig, kitlog.NewNopLogger())
	require.NoError(t, err)

	httpSrv := &http.Server{
		ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() {
		_ = httpSrv.Serve(tlsLis)
	}()
	defer func() {
		httpSrv.Close()
	}()

	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	cli := http.Client{Transport: httpTransport}

	resp, err := cli.Get(fmt.Sprintf("https://%s", tlsLis.Addr()))
	if err == nil {
		resp.Body.Close()
	}
	require.NoError(t, err)

	// Update our TLSConfig to require a client cert.
	tlsConfig.ClientAuth = "RequireAndVerifyClientCert"
	require.NoError(t, tlsLis.ApplyConfig(tlsConfig))

	// Close our idle connections so our next request forces a new dial.
	httpTransport.CloseIdleConnections()

	// Create a second connection which should now fail because we don't supply a
	resp, err = cli.Get(fmt.Sprintf("https://%s", tlsLis.Addr()))
	if err == nil {
		resp.Body.Close()
	}

	var urlError *url.Error
	require.ErrorAs(t, err, &urlError)
	require.Contains(t, urlError.Err.Error(), "tls:")
}
