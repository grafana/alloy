package harness

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/loki/pkg/push"
	promql_parser "github.com/prometheus/prometheus/promql/parser"
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
	LokiPushUrl  string            `alloy:"loki_push_url,attr"`
	LokiReceiver loki.LogsReceiver `alloy:"loki_receiver,attr"`
}

type Sink struct {
	opts component.Options
	args SinkArguments

	server   *httptest.Server
	lokirecv loki.LogsReceiver

	mux         sync.Mutex
	lokiEntries []loki.Entry
}

func NewSink(opts component.Options, args SinkArguments) (*Sink, error) {
	s := &Sink{
		opts:     opts,
		args:     args,
		lokirecv: loki.NewLogsReceiver(loki.WithComponentID(opts.ID)),
	}

	router := mux.NewRouter()
	router.HandleFunc(lokiPushPath, func(w http.ResponseWriter, r *http.Request) {
		var req push.PushRequest
		if err := util.ParseProtoReader(r.Context(), r.Body, int(r.ContentLength), math.MaxInt32, &req, util.RawSnappy); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
		LokiPushUrl:  s.server.URL + lokiPushPath,
		LokiReceiver: s.lokirecv,
	})

	return s, nil
}

var _ component.Component = (*Sink)(nil)

func (s *Sink) Run(ctx context.Context) error {
	defer s.server.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-s.lokirecv.Chan():
			s.mux.Lock()
			s.lokiEntries = append(s.lokiEntries, e)
			s.mux.Unlock()
		}
	}
}

func (s *Sink) Update(args component.Arguments) error {
	s.args = args.(SinkArguments)
	return nil
}

type snapshot struct {
	loki []loki.Entry
}

func (s *Sink) snapshot() snapshot {
	s.mux.Lock()
	defer s.mux.Unlock()

	entries := make([]loki.Entry, len(s.lokiEntries))
	copy(entries, s.lokiEntries)

	return snapshot{
		loki: entries,
	}
}
