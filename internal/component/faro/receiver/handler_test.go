package receiver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alecthomas/units"
	"github.com/grafana/agent/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/agent/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const emptyPayload = `{
	"traces": {
		"resourceSpans": []
	},
	"logs": [],
	"exceptions": [],
	"measurements": [],
	"meta": {}
}`

func TestMultipleExportersAllSucceed(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestMultipleExportersOneFails(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", true, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 1)
}

func TestMultipleExportersAllFail(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", true, nil}
		exporter2 = &testExporter{"exporter2", true, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestPayloadWithinLimit(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		MaxAllowedPayloadSize: units.Base2Bytes(len(emptyPayload)),
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestPayloadTooLarge(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		MaxAllowedPayloadSize: units.Base2Bytes(len(emptyPayload) - 1),
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestMissingAPIKey(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		APIKey: "fakekey",
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestInvalidAPIKey(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		APIKey: "fakekey",
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)
	req.Header.Set("x-api-key", "badkey")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestValidAPIKey(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		APIKey: "fakekey",
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)
	req.Header.Set("x-api-key", "fakekey")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestRateLimiter(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Rate:      1,
			BurstSize: 2,
		},
	})

	doRequest := func() *httptest.ResponseRecorder {
		req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	reqs := make([]*httptest.ResponseRecorder, 5)
	for i := range reqs {
		reqs[i] = doRequest()
	}

	// Only 1 request is allowed per second, with a burst of 2; meaning the third
	// request and beyond should be rejected.
	assert.Equal(t, http.StatusAccepted, reqs[0].Result().StatusCode)
	assert.Equal(t, http.StatusAccepted, reqs[1].Result().StatusCode)
	assert.Equal(t, http.StatusTooManyRequests, reqs[2].Result().StatusCode)
	assert.Equal(t, http.StatusTooManyRequests, reqs[3].Result().StatusCode)
	assert.Equal(t, http.StatusTooManyRequests, reqs[4].Result().StatusCode)
}

type testExporter struct {
	name     string
	broken   bool
	payloads []payload.Payload
}

func (te *testExporter) Name() string {
	return te.name
}

func (te *testExporter) Export(ctx context.Context, payload payload.Payload) error {
	if te.broken {
		return errors.New("this exporter is broken")
	}
	te.payloads = append(te.payloads, payload)
	return nil
}
