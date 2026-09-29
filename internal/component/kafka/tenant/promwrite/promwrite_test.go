package promwrite

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/util"
)

// Sink is a fake remote-write endpoint that records requests per tenant.
type Sink struct {
	mut      sync.Mutex
	requests []Request
	Status   []int // Status codes to return in order; 204 once exhausted.
}

type Request struct {
	Tenant string
	Req    prompb.WriteRequest
}

func (s *Sink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if len(s.Status) > 0 {
		code := s.Status[0]
		s.Status = s.Status[1:]
		if code/100 != 2 {
			w.WriteHeader(code)
			return
		}
	}

	compressed, _ := io.ReadAll(r.Body)
	raw, err := snappy.Decode(nil, compressed)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var req prompb.WriteRequest
	if err := req.Unmarshal(raw); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s.requests = append(s.requests, Request{Tenant: r.Header.Get(user.OrgIDHeaderName), Req: req})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Sink) Requests() []Request {
	s.mut.Lock()
	defer s.mut.Unlock()
	return append([]Request(nil), s.requests...)
}

func newComponent(t *testing.T, url string) *Component {
	var args Arguments
	args.Endpoint.SetToDefault()
	args.Endpoint.URL = url
	args.Endpoint.MinBackoff = time.Millisecond
	args.Endpoint.MaxBackoff = time.Millisecond
	args.Endpoint.MaxRetries = 3

	c, err := New(component.Options{
		ID:            "kafka.tenant_prometheus_write.test",
		Logger:        util.TestAlloyLogger(t).Slog(),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(component.Exports) {},
	}, args)
	require.NoError(t, err)
	return c
}

func appendSome(t *testing.T, app storage.Appender) {
	lbls := labels.FromStrings("__name__", "up", "job", "a")
	_, err := app.Append(0, lbls, 1000, 1)
	require.NoError(t, err)
	_, err = app.Append(0, lbls, 2000, 2)
	require.NoError(t, err)
	_, err = app.AppendExemplar(0, lbls, exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "x"), Value: 1, Ts: 1000})
	require.NoError(t, err)
}

func TestCommit_SendsTenantHeader(t *testing.T) {
	sink := &Sink{}
	srv := httptest.NewServer(sink)
	defer srv.Close()
	c := newComponent(t, srv.URL)

	app := c.Appender(user.InjectOrgID(context.Background(), "tenant-a"))
	appendSome(t, app)
	require.NoError(t, app.Commit())

	reqs := sink.Requests()
	require.Len(t, reqs, 1)
	require.Equal(t, "tenant-a", reqs[0].Tenant)
	require.Len(t, reqs[0].Req.Timeseries, 1, "samples of one series are grouped")
	require.Len(t, reqs[0].Req.Timeseries[0].Samples, 2)
	require.Len(t, reqs[0].Req.Timeseries[0].Exemplars, 1)
}

func TestCommit_MissingTenant(t *testing.T) {
	sink := &Sink{}
	srv := httptest.NewServer(sink)
	defer srv.Close()
	c := newComponent(t, srv.URL)

	app := c.Appender(context.Background())
	appendSome(t, app)
	require.ErrorContains(t, app.Commit(), "no tenant")
	require.Empty(t, sink.Requests())
}

func TestCommit_Retries5xx(t *testing.T) {
	sink := &Sink{Status: []int{http.StatusInternalServerError, http.StatusTooManyRequests}}
	srv := httptest.NewServer(sink)
	defer srv.Close()
	c := newComponent(t, srv.URL)

	app := c.Appender(user.InjectOrgID(context.Background(), "tenant-a"))
	appendSome(t, app)
	require.NoError(t, app.Commit())
	require.Len(t, sink.Requests(), 1)
}

func TestCommit_DoesNotRetry4xx(t *testing.T) {
	sink := &Sink{Status: []int{http.StatusBadRequest}}
	srv := httptest.NewServer(sink)
	defer srv.Close()
	c := newComponent(t, srv.URL)

	app := c.Appender(user.InjectOrgID(context.Background(), "tenant-a"))
	appendSome(t, app)
	require.ErrorContains(t, app.Commit(), "400")
	require.Empty(t, sink.Requests())
}

func TestRollback(t *testing.T) {
	sink := &Sink{}
	srv := httptest.NewServer(sink)
	defer srv.Close()
	c := newComponent(t, srv.URL)

	app := c.Appender(user.InjectOrgID(context.Background(), "tenant-a"))
	appendSome(t, app)
	require.NoError(t, app.Rollback())
	require.NoError(t, app.Commit())
	require.Empty(t, sink.Requests())
}
