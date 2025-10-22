package net

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func TestTargetServer(t *testing.T) {
	// dependencies
	reg := prometheus.NewRegistry()
	ts, err := NewTargetServer(util.TestLogger(t), "test_namespace", reg, &ServerConfig{})
	require.NoError(t, err)

	err = ts.MountAndRun(func(router *mux.Router) {
		router.Methods("GET").Path("/hello").Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	})
	require.NoError(t, err)
	defer ts.StopAndShutdown()

	// test mounted endpoint
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/hello", ts.HTTPListenAddr()), nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)

	// assert all metrics have the prefix applied
	metrics, err := reg.Gather()
	require.NoError(t, err)
	for _, m := range metrics {
		assert.True(t, strings.HasPrefix(m.GetName(), "test_namespace"), "expected %q to start with %q", m.GetName(), "test_namespace")
	}
}

func TestTargetServer_NilConfig(t *testing.T) {
	reg := prometheus.NewRegistry()
	ts, err := NewTargetServer(util.TestLogger(t), "test_namespace", reg, nil)
	require.NoError(t, err)

	err = ts.MountAndRun(func(router *mux.Router) {})
	require.NoError(t, err)
	defer ts.StopAndShutdown()

	require.Equal(t, "[::]:8080", ts.HTTPListenAddr())
	// not asserting over grpc port since a random should have been assigned
}
