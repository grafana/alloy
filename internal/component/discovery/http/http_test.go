package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	promhttp "github.com/prometheus/prometheus/discovery/http"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"gotest.tools/assert"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/syntax"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	url = "https://www.example.com:12345/foo"
	refresh_interval = "14s"
	basic_auth {
		username = "123"
		password = "456"
	}
	http_headers = {
		"foo" = ["foobar"],
	}
`
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
	assert.Equal(t, args.HTTPClientConfig.BasicAuth.Username, "123")

	header := args.HTTPClientConfig.HTTPHeaders.Headers["foo"][0]
	assert.Equal(t, "foobar", string(header))
}

func TestConvert(t *testing.T) {
	args := DefaultArguments
	u, err := url.Parse("https://www.example.com:12345/foo")
	require.NoError(t, err)
	args.URL = config.URL{URL: u}

	sd := args.Convert().(*promhttp.SDConfig)
	assert.Equal(t, "https://www.example.com:12345/foo", sd.URL)
	assert.Equal(t, model.Duration(60*time.Second), sd.RefreshInterval)
	assert.Equal(t, true, sd.HTTPClientConfig.EnableHTTP2)
}

func TestComponent(t *testing.T) {
	discovery.MaxUpdateFrequency = time.Second / 2
	endpointCalled := false
	var stateChanged atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpointCalled = true
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(200)
		// from https://prometheus.io/docs/prometheus/latest/http_sd/
		w.Write([]byte(`[
			{
				"targets": ["10.0.10.2:9100", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"],
				"labels": {
					"__meta_datacenter": "london",
					"__meta_prometheus_job": "node"
				}
			},
			{
				"targets": ["10.0.40.2:9100", "10.0.40.3:9100"],
				"labels": {
					"__meta_datacenter": "london",
					"__meta_prometheus_job": "alertmanager"
				}
			},
			{
				"targets": ["10.0.40.2:9093", "10.0.40.3:9093"],
				"labels": {
					"__meta_datacenter": "newyork",
					"__meta_prometheus_job": "alertmanager"
				}
			}
		]`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	var cancel func()
	component, err := discovery.NewFromConvertibleConfig(
		component.Options{
			OnStateChange: func(e component.Exports) {
				stateChanged.Store(true)
				args, ok := e.(discovery.Exports)
				assert.Equal(t, true, ok)
				assert.Equal(t, 8, len(args.Targets))
				cancel()
			},
			Registerer: prometheus.NewRegistry(),
			GetServiceData: func(name string) (any, error) {
				switch name {
				case livedebugging.ServiceName:
					return livedebugging.NewLiveDebugging(), nil
				default:
					return nil, fmt.Errorf("service %q does not exist", name)
				}
			},
		},
		Arguments{
			RefreshInterval:  time.Second,
			HTTPClientConfig: config.DefaultHTTPClientConfig,
			URL: config.URL{
				URL: u,
			},
		})
	assert.NilError(t, err)
	wg := sync.WaitGroup{}
	var ctx context.Context
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Second)
	wg.Add(1)
	go func() {
		err := component.Run(ctx)
		assert.NilError(t, err)
		wg.Done()
	}()
	wg.Wait()
	cancel()
	assert.Equal(t, true, endpointCalled)
	assert.Equal(t, true, stateChanged.Load())
}
