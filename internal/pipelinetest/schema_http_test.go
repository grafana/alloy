package pipelinetest

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateHTTPInputs(t *testing.T) {
	tests := []struct {
		name    string
		input   HTTPInputSchema
		wantErr string
	}{
		{name: "valid", input: HTTPInputSchema{URL: "http://127.0.0.1:1516/path"}},
		{name: "relative url", input: HTTPInputSchema{URL: "/path"}, wantErr: "absolute http or https URL"},
		{name: "unsupported scheme", input: HTTPInputSchema{URL: "tcp://127.0.0.1:1516"}, wantErr: "absolute http or https URL"},
		{name: "invalid status", input: HTTPInputSchema{URL: "http://127.0.0.1:1516", ExpectedStatus: 42}, wantErr: "not a valid HTTP status code"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHTTPInputs([]HTTPInputSchema{tc.input})
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestProduceHTTPInputs(t *testing.T) {
	type request struct {
		method, path, header, body string
	}
	var (
		mut sync.Mutex
		got []request
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mut.Lock()
		got = append(got, request{method: r.Method, path: r.URL.Path, header: r.Header.Get("X-Test"), body: string(body)})
		mut.Unlock()
		if r.URL.Path == "/created" {
			w.WriteHeader(http.StatusCreated)
		}
	}))
	t.Cleanup(srv.Close)

	err := produceHTTPInputs(t.Context(), []HTTPInputSchema{
		{URL: srv.URL + "/first", Headers: map[string]string{"X-Test": "a"}, Body: "one"},
		{URL: srv.URL + "/created", Method: http.MethodPut, Body: "two", ExpectedStatus: http.StatusCreated},
	})
	require.NoError(t, err)

	// Requests are sent in order, POST by default.
	mut.Lock()
	defer mut.Unlock()
	require.Equal(t, []request{
		{method: http.MethodPost, path: "/first", header: "a", body: "one"},
		{method: http.MethodPut, path: "/created", body: "two"},
	}, got)
}

func TestProduceHTTPInputsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad frame")
	}))
	t.Cleanup(srv.Close)

	err := produceHTTPInputs(t.Context(), []HTTPInputSchema{{URL: srv.URL}})
	require.ErrorContains(t, err, "want a 2xx status, got 400: bad frame")

	err = produceHTTPInputs(t.Context(), []HTTPInputSchema{{URL: srv.URL, ExpectedStatus: http.StatusNoContent}})
	require.ErrorContains(t, err, "want status 204, got 400")
}

// TestProduceHTTPInputsWaitsForListener checks that a request is retried while
// nothing listens on the port yet, since components start their listeners as
// they run.
func TestProduceHTTPInputsWaitsForListener(t *testing.T) {
	// Reserve a free port, then release it so the first attempts are refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	go func() {
		time.Sleep(300 * time.Millisecond)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		srv.Listener = l
		srv.Start()
	}()

	require.NoError(t, produceHTTPInputs(t.Context(), []HTTPInputSchema{{URL: "http://" + addr + "/push"}}))
}
