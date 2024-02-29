package app_agent_receiver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/client_golang/prometheus"
)

const PAYLOAD = `
{
  "traces": {
    "resourceSpans": []
  },
  "logs": [],
  "exceptions": [],
  "measurements": [],
  "meta": {}
}
`

type TestExporter struct {
	name     string
	broken   bool
	payloads []Payload
}

func (te *TestExporter) Name() string {
	return te.name
}

func (te *TestExporter) Export(ctx context.Context, payload Payload) error {
	if te.broken {
		return errors.New("this exporter is broken")
	}
	te.payloads = append(te.payloads, payload)
	return nil
}

func TestMultipleExportersAllSucceed(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))

	reg := prometheus.NewRegistry()

	require.NoError(t, err)

	exporter1 := TestExporter{
		name:     "exporter1",
		broken:   false,
		payloads: []Payload{},
	}
	exporter2 := TestExporter{
		name:     "exporter2",
		broken:   false,
		payloads: []Payload{},
	}

	conf := &Config{}

	fr := NewAppAgentReceiverHandler(conf, []AppAgentReceiverExporter{&exporter1, &exporter2}, reg)
	handler := fr.HTTPHandler(log.NewNopLogger())

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)

	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestMultipleExportersOneFails(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))

	require.NoError(t, err)

	reg := prometheus.NewRegistry()

	exporter1 := TestExporter{
		name:     "exporter1",
		broken:   true,
		payloads: []Payload{},
	}
	exporter2 := TestExporter{
		name:     "exporter2",
		broken:   false,
		payloads: []Payload{},
	}

	conf := &Config{}

	fr := NewAppAgentReceiverHandler(conf, []AppAgentReceiverExporter{&exporter1, &exporter2}, reg)
	handler := fr.HTTPHandler(log.NewNopLogger())

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	metrics, err := reg.Gather()
	require.NoError(t, err)

	metric := metrics[0]
	require.Equal(t, "app_agent_receiver_exporter_errors_total", *metric.Name)
	require.Len(t, metric.Metric, 1)
	require.Equal(t, 1.0, *metric.Metric[0].Counter.Value)
	require.Len(t, metric.Metric[0].Label, 1)
	require.Equal(t, *metric.Metric[0].Label[0].Value, "exporter1")
	require.Len(t, metrics, 1)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 1)
}

func TestMultipleExportersAllFail(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))

	reg := prometheus.NewRegistry()

	require.NoError(t, err)

	exporter1 := TestExporter{
		name:     "exporter1",
		broken:   true,
		payloads: []Payload{},
	}
	exporter2 := TestExporter{
		name:     "exporter2",
		broken:   true,
		payloads: []Payload{},
	}

	conf := &Config{}

	fr := NewAppAgentReceiverHandler(conf, []AppAgentReceiverExporter{&exporter1, &exporter2}, reg)
	handler := fr.HTTPHandler(log.NewNopLogger())

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	metrics, err := reg.Gather()
	require.NoError(t, err)

	require.Len(t, metrics, 1)
	metric := metrics[0]

	require.Equal(t, "app_agent_receiver_exporter_errors_total", *metric.Name)
	require.Len(t, metric.Metric, 2)
	require.Equal(t, 1.0, *metric.Metric[0].Counter.Value)
	require.Equal(t, 1.0, *metric.Metric[1].Counter.Value)
	require.Len(t, metric.Metric[0].Label, 1)
	require.Len(t, metric.Metric[1].Label, 1)
	require.Equal(t, *metric.Metric[0].Label[0].Value, "exporter1")
	require.Equal(t, *metric.Metric[1].Label[0].Value, "exporter2")
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestNoContentLengthLimitSet(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))
	require.NoError(t, err)
	reg := prometheus.NewRegistry()

	conf := &Config{}

	req.ContentLength = 89348593894

	fr := NewAppAgentReceiverHandler(conf, []AppAgentReceiverExporter{}, reg)
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
}

func TestLargePayload(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))
	require.NoError(t, err)
	reg := prometheus.NewRegistry()

	conf := &Config{
		Server: ServerConfig{
			MaxAllowedPayloadSize: 10,
		},
	}

	fr := NewAppAgentReceiverHandler(conf, []AppAgentReceiverExporter{}, reg)
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Result().StatusCode)
}

func TestAPIKeyRequiredButNotProvided(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))

	if err != nil {
		t.Fatal(err)
	}

	conf := &Config{
		Server: ServerConfig{
			APIKey: "foo",
		},
	}

	fr := NewAppAgentReceiverHandler(conf, nil, prometheus.NewRegistry())
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
}

func TestAPIKeyWrong(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))
	req.Header.Set("x-api-key", "bar")

	if err != nil {
		t.Fatal(err)
	}

	conf := &Config{
		Server: ServerConfig{
			APIKey: "foo",
		},
	}

	fr := NewAppAgentReceiverHandler(conf, nil, prometheus.NewRegistry())
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
}

func TestAPIKeyCorrect(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))
	req.Header.Set("x-api-key", "foo")

	if err != nil {
		t.Fatal(err)
	}

	conf := &Config{
		Server: ServerConfig{
			APIKey: "foo",
		},
	}

	fr := NewAppAgentReceiverHandler(conf, nil, prometheus.NewRegistry())
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
}

func TestRateLimiterNoReject(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))

	if err != nil {
		t.Fatal(err)
	}

	conf := &Config{
		Server: ServerConfig{
			RateLimiting: RateLimitingConfig{
				Burstiness: 10,
				RPS:        10,
				Enabled:    true,
			},
		},
	}

	fr := NewAppAgentReceiverHandler(conf, nil, prometheus.NewRegistry())
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
}

func TestRateLimiterReject(t *testing.T) {
	conf := &Config{
		Server: ServerConfig{
			RateLimiting: RateLimitingConfig{
				Burstiness: 2,
				RPS:        1,
				Enabled:    true,
			},
		},
	}

	fr := NewAppAgentReceiverHandler(conf, nil, prometheus.NewRegistry())
	handler := fr.HTTPHandler(nil)

	makeRequest := func() *httptest.ResponseRecorder {
		req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	r1 := makeRequest()
	r2 := makeRequest()
	r3 := makeRequest()

	require.Equal(t, http.StatusAccepted, r1.Result().StatusCode)
	require.Equal(t, http.StatusAccepted, r2.Result().StatusCode)
	require.Equal(t, http.StatusTooManyRequests, r3.Result().StatusCode)
}

func TestRateLimiterDisabled(t *testing.T) {
	req, err := http.NewRequest("POST", "/collect", bytes.NewBuffer([]byte(PAYLOAD)))

	if err != nil {
		t.Fatal(err)
	}

	conf := &Config{
		Server: ServerConfig{
			RateLimiting: RateLimitingConfig{
				Burstiness: 0,
				RPS:        0,
				Enabled:    false,
			},
		},
	}

	fr := NewAppAgentReceiverHandler(conf, nil, prometheus.NewRegistry())
	handler := fr.HTTPHandler(nil)

	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
}
