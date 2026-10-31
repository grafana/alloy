package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	commonconfig "github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const singleAlertWebhook = `{
  "receiver": "alloy",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "HighCPU",
        "instance": "server01"
      },
      "annotations": {
        "summary": "CPU usage is high"
      },
      "startsAt": "2026-08-25T08:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus/graph",
      "fingerprint": "webhook-only"
    }
  ],
  "groupLabels": {},
  "commonLabels": {},
  "commonAnnotations": {},
  "externalURL": "",
  "version": "4",
  "groupKey": ""
}`

func TestRegistrationAndConfiguration(t *testing.T) {
	registration, found := component.Get("prometheus.alertmanager.relay")
	require.True(t, found)
	require.True(t, registration.Community)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
      listen_address        = "0.0.0.0"
      listen_port           = 5001
      webhook_path          = "/alerts"
      max_request_body_size = "2MiB"

      endpoint {
        url     = "https://alertmanager.example.com/api/v2/alerts"
        timeout = "15s"

        tls_config {
          insecure_skip_verify = true
        }
      }
    `), &args))
	require.Equal(t, "0.0.0.0", args.ListenAddress)
	require.Equal(t, 5001, args.ListenPort)
	require.Equal(t, "/alerts", args.WebhookPath)
	require.Equal(t, 2*units.MiB, args.MaxRequestBodySize)
	require.Equal(t, 15*time.Second, args.Endpoint.Timeout)
	require.True(t, args.Endpoint.HTTPClientConfig.TLSConfig.InsecureSkipVerify)
}

func TestWebhookConversion(t *testing.T) {
	var (
		requestCount atomic.Int64
		gotMethod    string
		gotPath      string
		gotType      string
		gotBody      []byte
	)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotType = r.Header.Get("Content-Type")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	c := newTestComponent(t, testArguments(destination.URL))
	response := serveWebhook(c, http.MethodPost, singleAlertWebhook)
	require.Equal(t, http.StatusOK, response.Code)
	require.EqualValues(t, 1, requestCount.Load())
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, defaultAlertsPath, gotPath)
	require.Equal(t, "application/json", gotType)

	var actual []map[string]any
	require.NoError(t, json.Unmarshal(gotBody, &actual))
	require.Equal(t, []map[string]any{
		{
			"labels": map[string]any{
				"alertname": "HighCPU",
				"instance":  "server01",
			},
			"annotations": map[string]any{
				"summary": "CPU usage is high",
			},
			"startsAt":     "2026-08-25T08:00:00.000Z",
			"endsAt":       "0001-01-01T00:00:00.000Z",
			"generatorURL": "http://prometheus/graph",
		},
	}, actual)
	require.NotContains(t, string(gotBody), `"status"`)
	require.NotContains(t, string(gotBody), `"fingerprint"`)
}

func TestMultipleAlertsForwardedAsOneRequest(t *testing.T) {
	var (
		requestCount atomic.Int64
		alertCount   int
	)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		var alerts []map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&alerts))
		alertCount = len(alerts)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	payload := `{
      "receiver":"alloy",
      "alerts":[
        {"status":"firing","labels":{"alertname":"One"},"annotations":{},"startsAt":"2026-08-25T08:00:00Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":""},
        {"status":"resolved","labels":{"alertname":"Two"},"annotations":{"summary":"done"},"startsAt":"2026-08-25T08:01:00Z","endsAt":"2026-08-25T08:02:00Z","generatorURL":"http://prometheus/graph"}
      ],
      "version":"4"
    }`
	c := newTestComponent(t, testArguments(destination.URL+defaultAlertsPath))
	response := serveWebhook(c, http.MethodPost, payload)
	require.Equal(t, http.StatusOK, response.Code)
	require.EqualValues(t, 1, requestCount.Load())
	require.Equal(t, 2, alertCount)
}

func TestRejectedIncomingRequests(t *testing.T) {
	var destinationRequests atomic.Int64
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	tests := []struct {
		name       string
		method     string
		body       string
		mutateArgs func(*Arguments)
		wantStatus int
	}{
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			body:       singleAlertWebhook,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "malformed JSON",
			method:     http.MethodPost,
			body:       `{"alerts":[`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "multiple JSON values",
			method:     http.MethodPost,
			body:       `{"alerts":[]} {}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty alert list",
			method:     http.MethodPost,
			body:       `{"receiver":"alloy","alerts":[],"version":"4"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing labels",
			method:     http.MethodPost,
			body:       `{"alerts":[{"status":"firing","annotations":{},"startsAt":"2026-08-25T08:00:00Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":""}]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "request body too large",
			method: http.MethodPost,
			body:   singleAlertWebhook,
			mutateArgs: func(args *Arguments) {
				args.MaxRequestBodySize = units.Base2Bytes(len(singleAlertWebhook) - 1)
			},
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := testArguments(destination.URL)
			if tc.mutateArgs != nil {
				tc.mutateArgs(&args)
			}
			c := newTestComponent(t, args)
			response := serveWebhook(c, tc.method, tc.body)
			require.Equal(t, tc.wantStatus, response.Code)
			if tc.wantStatus == http.StatusMethodNotAllowed {
				require.Equal(t, http.MethodPost, response.Header().Get("Allow"))
			}
		})
	}
	require.Zero(t, destinationRequests.Load())
}

func TestDestinationFailures(t *testing.T) {
	tests := []struct {
		name              string
		destinationStatus int
		unavailable       bool
		wantStatus        int
		wantFailureReason string
	}{
		{name: "destination returns 400", destinationStatus: http.StatusBadRequest, wantStatus: http.StatusBadGateway, wantFailureReason: failureStatus4xx},
		{name: "destination returns 500", destinationStatus: http.StatusInternalServerError, wantStatus: http.StatusBadGateway, wantFailureReason: failureStatus5xx},
		{name: "destination unavailable", unavailable: true, wantStatus: http.StatusBadGateway, wantFailureReason: failureConnection},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var endpoint string
			if tc.unavailable {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				endpoint = "http://" + listener.Addr().String() + defaultAlertsPath
				require.NoError(t, listener.Close())
			} else {
				destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.destinationStatus)
				}))
				defer destination.Close()
				endpoint = destination.URL + defaultAlertsPath
			}

			registry := prometheus.NewRegistry()
			c := newTestComponentWithRegistry(t, testArguments(endpoint), registry)
			response := serveWebhook(c, http.MethodPost, singleAlertWebhook)
			require.Equal(t, tc.wantStatus, response.Code)
			require.Equal(t, float64(1), metricValue(t, registry, "prometheus_alertmanager_relay_outbound_request_failures_total", map[string]string{"reason": tc.wantFailureReason}))
		})
	}
}

func TestDestinationTimeout(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	args := testArguments(destination.URL)
	args.Endpoint.Timeout = 25 * time.Millisecond
	registry := prometheus.NewRegistry()
	c := newTestComponentWithRegistry(t, args, registry)
	response := serveWebhook(c, http.MethodPost, singleAlertWebhook)
	require.Equal(t, http.StatusGatewayTimeout, response.Code)
	require.Equal(t, float64(1), metricValue(t, registry, "prometheus_alertmanager_relay_outbound_request_failures_total", map[string]string{"reason": failureTimeout}))
}

func TestTLSVerification(t *testing.T) {
	destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	t.Run("enabled by default", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		c := newTestComponentWithRegistry(t, testArguments(destination.URL), registry)
		response := serveWebhook(c, http.MethodPost, singleAlertWebhook)
		require.Equal(t, http.StatusBadGateway, response.Code)
		require.Equal(t, float64(1), metricValue(t, registry, "prometheus_alertmanager_relay_outbound_request_failures_total", map[string]string{"reason": failureTLS}))
	})

	t.Run("can be explicitly disabled", func(t *testing.T) {
		args := testArguments(destination.URL)
		args.Endpoint.HTTPClientConfig.TLSConfig.InsecureSkipVerify = true
		c := newTestComponent(t, args)
		response := serveWebhook(c, http.MethodPost, singleAlertWebhook)
		require.Equal(t, http.StatusOK, response.Code)
	})
}

func TestConcurrentIncomingRequests(t *testing.T) {
	const requestTotal = 32
	var destinationRequests atomic.Int64
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	c := newTestComponent(t, testArguments(destination.URL))
	var wg sync.WaitGroup
	statuses := make(chan int, requestTotal)
	for range requestTotal {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- serveWebhook(c, http.MethodPost, singleAlertWebhook).Code
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		require.Equal(t, http.StatusOK, status)
	}
	require.EqualValues(t, requestTotal, destinationRequests.Load())
}

func TestGracefulShutdownWaitsForActiveRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args := testArguments(destination.URL)
	args.ListenPort = port
	c := newTestComponent(t, args)

	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(runCtx) }()
	require.Eventually(t, func() bool {
		return c.CurrentHealth().Health == component.HealthTypeHealthy
	}, time.Second, 10*time.Millisecond)

	requestDone := make(chan int, 1)
	go func() {
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/webhook", port), "application/json", bytes.NewBufferString(singleAlertWebhook))
		if err != nil {
			requestDone <- 0
			return
		}
		defer resp.Body.Close()
		requestDone <- resp.StatusCode
	}()
	<-requestStarted
	cancel()

	select {
	case err := <-runDone:
		t.Fatalf("component stopped before the active request completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseRequest)
	require.Equal(t, http.StatusOK, <-requestDone)
	require.NoError(t, <-runDone)
}

func TestListenerBindFailureIsUnhealthy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	args := testArguments(destination.URL)
	args.ListenPort = port
	c := newTestComponent(t, args)
	runCtx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- c.Run(runCtx) }()

	require.Eventually(t, func() bool {
		return c.CurrentHealth().Health == component.HealthTypeUnhealthy
	}, time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, <-runDone)
}

func TestSharedHTTPClientConfiguration(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "alloy", username)
		require.Equal(t, "secret", password)
		require.Equal(t, "tenant-a", r.Header.Get("X-Scope-OrgID"))
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	args := testArguments(destination.URL)
	args.Endpoint.HTTPClientConfig.BasicAuth = &commonconfig.BasicAuth{
		Username: "alloy",
		Password: alloytypes.Secret("secret"),
	}
	args.Endpoint.HTTPClientConfig.HTTPHeaders = &commonconfig.Headers{
		Headers: map[string][]alloytypes.Secret{
			"X-Scope-OrgID": {alloytypes.Secret("tenant-a")},
		},
	}
	c := newTestComponent(t, args)
	require.Equal(t, http.StatusOK, serveWebhook(c, http.MethodPost, singleAlertWebhook).Code)
}

func testArguments(endpoint string) Arguments {
	var args Arguments
	args.SetToDefault()
	args.Endpoint.URL = endpoint
	return args
}

func newTestComponent(t *testing.T, args Arguments) *Component {
	t.Helper()
	return newTestComponentWithRegistry(t, args, prometheus.NewRegistry())
}

func newTestComponentWithRegistry(t *testing.T, args Arguments, registry *prometheus.Registry) *Component {
	t.Helper()
	c, err := New(component.Options{
		ID:         "prometheus.alertmanager.relay.test",
		Logger:     util.TestLogger(t),
		Tracer:     noop.NewTracerProvider(),
		Registerer: registry,
	}, args)
	require.NoError(t, err)
	return c
}

func serveWebhook(c *Component, method, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/webhook", bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	c.handleWebhook(response, request)
	return response
}

func metricValue(t *testing.T, registry *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			matches := true
			for labelName, labelValue := range labels {
				found := false
				for _, pair := range metric.Label {
					if pair.GetName() == labelName && pair.GetValue() == labelValue {
						found = true
						break
					}
				}
				if !found {
					matches = false
					break
				}
			}
			if matches {
				if metric.Counter != nil {
					return metric.Counter.GetValue()
				}
				if metric.Gauge != nil {
					return metric.Gauge.GetValue()
				}
			}
		}
	}
	t.Fatalf("metric %s with labels %v not found", name, labels)
	return 0
}
