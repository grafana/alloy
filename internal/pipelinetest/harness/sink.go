package harness

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	promql_parser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
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
	LokiPushUrl        string             `alloy:"loki_push_url,attr"`
	LokiReceiver       loki.LogsReceiver  `alloy:"loki_receiver,attr"`
	PrometheusReceiver storage.Appendable `alloy:"prometheus_receiver,attr"`
}

// PrometheusSample is one sample captured by the sink. Histogram is set only for
// native histogram samples, in which case Value is unset.
type PrometheusSample struct {
	Labels    labels.Labels
	Timestamp time.Time
	Value     float64
	Histogram *histogram.FloatHistogram
}

type Sink struct {
	opts component.Options
	args SinkArguments

	server   *httptest.Server
	lokirecv loki.LogsReceiver
	promrecv storage.Appendable

	mux         sync.Mutex
	lokiEntries []loki.Entry
	promSamples []PrometheusSample
}

func NewSink(opts component.Options, args SinkArguments) (*Sink, error) {
	s := &Sink{
		opts:     opts,
		args:     args,
		lokirecv: loki.NewLogsReceiver(loki.WithComponentID(opts.ID)),
	}

	// An Interceptor with no next Appendable terminates the chain, so appended
	// samples land in the snapshot and go no further.
	s.promrecv = alloyprom.NewInterceptor(
		nil,
		alloyprom.WithComponentID(opts.ID),
		alloyprom.WithAppendHook(func(_ storage.SeriesRef, l labels.Labels, t int64, v float64, _ storage.Appender) (storage.SeriesRef, error) {
			s.appendPrometheusSample(PrometheusSample{
				Labels:    l.Copy(),
				Timestamp: timestampToTime(t),
				Value:     v,
			})
			return 0, nil
		}),
		alloyprom.WithHistogramHook(func(_ storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, _ storage.Appender) (storage.SeriesRef, error) {
			s.appendPrometheusSample(PrometheusSample{
				Labels:    l.Copy(),
				Timestamp: timestampToTime(t),
				Histogram: toFloatHistogram(h, fh),
			})
			return 0, nil
		}),
	)

	router := mux.NewRouter()
	router.HandleFunc(lokiPushPath, func(w http.ResponseWriter, r *http.Request) {
		var req push.PushRequest
		if err := util.ParseProtoReader(r.Context(), r.Body, int(r.ContentLength), math.MaxInt32, &req, util.RawSnappy); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s.mux.Lock()
		for _, stream := range req.Streams {
			labels, err := promql_parser.NewParser(promql_parser.Options{}).ParseMetric(stream.Labels)
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
		LokiPushUrl:        s.server.URL + lokiPushPath,
		LokiReceiver:       s.lokirecv,
		PrometheusReceiver: s.promrecv,
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

func (s *Sink) appendPrometheusSample(sample PrometheusSample) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.promSamples = append(s.promSamples, sample)
}

type snapshot struct {
	loki       []loki.Entry
	prometheus []PrometheusSample
}

func (s *Sink) snapshot() snapshot {
	s.mux.Lock()
	defer s.mux.Unlock()

	entries := make([]loki.Entry, len(s.lokiEntries))
	copy(entries, s.lokiEntries)

	samples := make([]PrometheusSample, len(s.promSamples))
	copy(samples, s.promSamples)

	return snapshot{
		loki:       entries,
		prometheus: samples,
	}
}

// timestampToTime converts a Prometheus millisecond timestamp to a time.Time.
func timestampToTime(t int64) time.Time {
	return time.UnixMilli(t).UTC()
}

// toFloatHistogram normalises the two histogram representations an Appender can
// receive into the float form, so assertions only deal with one type.
func toFloatHistogram(h *histogram.Histogram, fh *histogram.FloatHistogram) *histogram.FloatHistogram {
	if fh != nil {
		return fh.Copy()
	}
	if h != nil {
		return h.ToFloat(nil)
	}
	return nil
}
