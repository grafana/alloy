package harness

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/loki/pkg/push"
	promql_parser "github.com/prometheus/prometheus/promql/parser"
	"github.com/stretchr/testify/require"
)

const (
	lokiPushPath = "/loki/api/v1/push"
)

func init() {
	component.Register(component.Registration{
		Name:      "pipelinetest.sink",
		Stability: featuregate.StabilityExperimental,
		Args:      SinkArguments{},
		Exports:   SinkExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSink(opts, args.(SinkArguments))
		},
	})
}

type SinkArguments struct{}

type SinkExports struct {
	LokiPushUrl string `alloy:"loki_push_url,attr"`
}

type Sink struct {
	opts component.Options
	args SinkArguments

	server      *httptest.Server
	mux         sync.Mutex
	lokiEntries []loki.Entry
}

func NewSink(opts component.Options, args SinkArguments) (*Sink, error) {
	s := &Sink{
		opts: opts,
		args: args,
	}

	router := mux.NewRouter()
	router.HandleFunc(lokiPushPath, func(w http.ResponseWriter, r *http.Request) {
		var req push.PushRequest
		if err := util.ParseProtoReader(r.Context(), r.Body, int(r.ContentLength), math.MaxInt32, &req, util.RawSnappy); err != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		s.mux.Lock()
		for _, stream := range req.Streams {
			labels, err := promql_parser.ParseMetric(stream.Labels)
			if err != nil {
				s.mux.Unlock()
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			labelSet := util.MapToModelLabelSet(labels.Map())
			for _, entry := range stream.Entries {
				s.lokiEntries = append(s.lokiEntries, loki.NewEntry(labelSet, entry))
			}
		}
		s.mux.Unlock()

		w.WriteHeader(http.StatusNoContent)
	}).Methods(http.MethodPost)

	s.server = httptest.NewServer(router)

	s.opts.OnStateChange(SinkExports{
		LokiPushUrl: s.server.URL + lokiPushPath,
	})

	return s, nil
}

var _ component.Component = (*Sink)(nil)

func (s *Sink) Run(ctx context.Context) error {
	defer s.server.Close()

	<-ctx.Done()
	return nil
}

func (s *Sink) Update(args component.Arguments) error {
	s.args = args.(SinkArguments)
	return nil
}

func (s *Sink) AssertEntries(t *testing.T, want ...loki.Entry) {
	t.Helper()

	require.Eventually(t, func() bool {
		return containsAllEntries(s.entries(), want)
	}, time.Second, 50*time.Millisecond)
}

func (s *Sink) entries() []loki.Entry {
	s.mux.Lock()
	defer s.mux.Unlock()

	out := make([]loki.Entry, len(s.lokiEntries))
	copy(out, s.lokiEntries)
	return out
}

func containsAllEntries(got, want []loki.Entry) bool {
	for _, w := range want {
		if slices.IndexFunc(got, func(e loki.Entry) bool {
			return equalEntry(e, w)
		}) < 0 {
			return false
		}
	}
	return true
}

func equalEntry(got, want loki.Entry) bool {
	return reflect.DeepEqual(got.Labels, want.Labels) &&
		got.Timestamp.Equal(want.Timestamp) &&
		got.Line == want.Line &&
		reflect.DeepEqual(got.StructuredMetadata, want.StructuredMetadata)
}
