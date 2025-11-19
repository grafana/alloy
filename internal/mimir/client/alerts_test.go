package client

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-kit/log"
	alertmgr_cfg "github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"
)

func TestMimirClient_CreateAlertmanagerConfigs(t *testing.T) {
	requestCh := make(chan *http.Request, 1)
	bodyCh := make(chan []byte, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCh <- r
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		bodyCh <- body
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "success")
	}))
	defer ts.Close()

	clientConfig := Config{
		Address: ts.URL,
	}

	client, err := New(log.NewNopLogger(), clientConfig, nil)
	require.NoError(t, err)

	// This Alertmanager config was copied from:
	// https://github.com/prometheus/alertmanager/blob/v0.28.1/config/testdata/conf.good.yml
	config, err := alertmgr_cfg.LoadFile("testdata/alertmanager/conf.good.yml")
	require.NoError(t, err)
	require.NotNil(t, config)

	templateFiles := map[string]string{
		"template1.tmpl": "{{ range .Alerts }}Alert: {{ .Summary }}{{ end }}",
		"template2.tmpl": "{{ .CommonLabels.alertname }}",
	}

	ctx := t.Context()
	err = client.CreateAlertmanagerConfigs(ctx, config, templateFiles)
	require.NoError(t, err)

	// Verify the request
	req := <-requestCh
	require.Equal(t, "POST", req.Method)
	require.Equal(t, "/api/v1/alerts", req.URL.Path)

	// Verify the request body
	body := <-bodyCh

	// Load expected response from test data file
	expectedResponseBytes, err := os.ReadFile("testdata/alertmanager/response.good.yml")
	require.NoError(t, err)
	expectedResponse := string(expectedResponseBytes)

	require.Equal(t, expectedResponse, string(body))
}
