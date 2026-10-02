package prometheus_test

import (
	"context"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/appenders/adapter"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/service/labelstore"
)

// The tests in this file run the same batches of samples through the V1
// (Appender) and V2 (AppenderV2) paths of a pipeline, and assert that both
// paths leave identical data in the destination storages and return identical
// refs to the caller.

// paritySample is one sample together with everything a scraper may attach to
// it.
type paritySample struct {
	lbls      labels.Labels
	st, t     int64
	v         float64
	h         *histogram.Histogram
	fh        *histogram.FloatHistogram
	exemplars []exemplar.Exemplar
	meta      metadata.Metadata
}

func (s paritySample) isHistogram() bool { return s.h != nil || s.fh != nil }

// parityPipeline is a pipeline under test. build is called once per path, so
// that the V1 and V2 runs don't share any state.
type parityPipeline struct {
	name  string
	build func(t *testing.T) (entry storage.AppendableV2, dests []*recordingStorage, between func(batch int))
}

func TestAppenderV1V2Parity(t *testing.T) {
	for _, p := range parityPipelines() {
		t.Run(p.name, func(t *testing.T) {
			batches := parityBatches()

			v1Entry, v1Dests, v1Between := p.build(t)
			v1Refs := appendBatchesV1(t, v1Entry.(storage.Appendable), batches, v1Between)

			v2Entry, v2Dests, v2Between := p.build(t)
			v2Refs := appendBatchesV2(t, v2Entry, batches, v2Between)

			require.Equal(t, v1Refs, v2Refs, "refs returned to the caller differ")
			require.Len(t, v2Dests, len(v1Dests))
			for i := range v1Dests {
				require.NotEmpty(t, v1Dests[i].series, "destination %d received no data", i)
				require.Equal(t, v1Dests[i].series, v2Dests[i].series, "data in destination %d differs", i)
			}
		})
	}
}

func parityPipelines() []parityPipeline {
	directFanout := func(n int) func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
		return func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
			dests, children := newRecordingStorages(n)
			return prometheus.NewFanout(children, "test", promclient.NewRegistry(), nil), dests, nil
		}
	}

	// labelStoreFanout mirrors the default production setup: the label store
	// is enabled, and the fanout's children are prometheus.remote_write
	// receivers, which translate global refs to their own local refs.
	labelStoreFanout := func(n int) func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
		return func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
			ls := labelstore.New(nil, promclient.NewRegistry(), true)
			dests := make([]*recordingStorage, n)
			children := make([]storage.AppendableV2, n)
			for i := range n {
				dests[i] = newRecordingStorage()
				children[i] = remotewrite.NewInterceptor("rw"+string(rune('a'+i)), &atomic.Bool{}, noopDebugDataPublisher{}, ls, dests[i])
			}
			return prometheus.NewFanout(children, "test", promclient.NewRegistry(), ls), dests, nil
		}
	}

	return []parityPipeline{
		{name: "fanout with one child", build: directFanout(1)},
		{name: "fanout with two children", build: directFanout(2)},
		{name: "fanout with one remote_write and label store", build: labelStoreFanout(1)},
		{name: "fanout with two remote_writes and label store", build: labelStoreFanout(2)},
		{
			name: "fanout changing from two children to one",
			build: func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
				dests, children := newRecordingStorages(2)
				fanout := prometheus.NewFanout(children, "test", promclient.NewRegistry(), nil)
				return fanout, dests, func(batch int) {
					if batch == 1 {
						fanout.UpdateChildren(children[:1])
					}
				}
			},
		},
		{
			name: "fanout changing from one child to two",
			build: func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
				dests, children := newRecordingStorages(2)
				fanout := prometheus.NewFanout(children[:1], "test", promclient.NewRegistry(), nil)
				return fanout, dests, func(batch int) {
					if batch == 1 {
						fanout.UpdateChildren(children)
					}
				}
			},
		},
		{
			// Hook-less Interceptors use the native interceptappenderV2.
			name: "interceptors without hooks around a fanout",
			build: func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
				dests, children := newRecordingStorages(2)
				for i := range children {
					children[i] = prometheus.NewInterceptor(children[i])
				}
				fanout := prometheus.NewFanout(children, "test", promclient.NewRegistry(), nil)
				return prometheus.NewInterceptor(fanout), dests, nil
			},
		},
		{
			// Interceptors with only V1 hooks fall back to adapting their V1
			// appender, so the hooks must still run on the V2 path. The hooks
			// record each call in the destination, so a skipped hook shows up
			// as a data difference.
			name: "interceptor with V1 hooks in front of a fanout",
			build: func(t *testing.T) (storage.AppendableV2, []*recordingStorage, func(int)) {
				dests, children := newRecordingStorages(2)
				fanout := prometheus.NewFanout(children, "test", promclient.NewRegistry(), nil)
				hookCalls := newRecordingStorage()
				return prometheus.NewInterceptor(fanout,
					prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
						_, _ = hookCalls.record(0, l, parityEvent{kind: "append_hook", t: t, v: v})
						return next.Append(ref, l, t, v)
					}),
					prometheus.WithHistogramHook(func(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
						_, _ = hookCalls.record(0, l, parityEvent{kind: "histogram_hook", t: t, h: h, fh: fh})
						return next.AppendHistogram(ref, l, t, h, fh)
					}),
					prometheus.WithExemplarHook(func(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error) {
						_, _ = hookCalls.record(0, l, parityEvent{kind: "exemplar_hook", e: e})
						return next.AppendExemplar(ref, l, e)
					}),
					prometheus.WithMetadataHook(func(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error) {
						_, _ = hookCalls.record(0, l, parityEvent{kind: "metadata_hook", m: m})
						return next.UpdateMetadata(ref, l, m)
					}),
					prometheus.WithSTZeroSampleHook(func(ref storage.SeriesRef, l labels.Labels, t, st int64, next storage.Appender) (storage.SeriesRef, error) {
						_, _ = hookCalls.record(0, l, parityEvent{kind: "st_hook", t: t, st: st})
						return next.AppendSTZeroSample(ref, l, t, st)
					}),
				), append(dests, hookCalls), nil
			},
		},
	}
}

func parityBatches() [][]paritySample {
	counter := labels.FromStrings("__name__", "requests_total", "job", "test")
	gauge := labels.FromStrings("__name__", "temperature", "job", "test")
	hist := labels.FromStrings("__name__", "latency", "job", "test")
	floatHist := labels.FromStrings("__name__", "size", "job", "test")

	counterMeta := metadata.Metadata{Type: "counter", Help: "Requests.", Unit: ""}
	gaugeMeta := metadata.Metadata{Type: "gauge", Help: "Temperature.", Unit: "celsius"}
	histMeta := metadata.Metadata{Type: "histogram", Help: "Latency.", Unit: "seconds"}

	batch := func(ts int64, i int) []paritySample {
		return []paritySample{
			{
				lbls: counter, st: 500, t: ts, v: float64(10 * i),
				exemplars: []exemplar.Exemplar{{Labels: labels.FromStrings("trace_id", "abc"), Value: 1, Ts: ts - 1, HasTs: true}},
				meta:      counterMeta,
			},
			{lbls: gauge, t: ts, v: float64(i), meta: gaugeMeta},
			{lbls: hist, st: 500, t: ts, h: tsdbutil.GenerateTestHistogram(int64(i)), meta: histMeta},
			{lbls: floatHist, t: ts, fh: tsdbutil.GenerateTestFloatHistogram(int64(i))},
		}
	}
	return [][]paritySample{batch(1000, 1), batch(2000, 2), batch(3000, 3)}
}

// appendBatchesV1 appends batches the way the Prometheus V1 scrape loop does:
// the ST zero sample first, then the sample, then exemplars and metadata. The
// ref returned by each call is passed to the next one, and the final ref is
// cached per series for the next batch.
func appendBatchesV1(t *testing.T, entry storage.Appendable, batches [][]paritySample, between func(int)) [][]storage.SeriesRef {
	cache := map[string]storage.SeriesRef{}
	var out [][]storage.SeriesRef
	for i, batch := range batches {
		if between != nil {
			between(i)
		}
		app := entry.Appender(t.Context())
		var refs []storage.SeriesRef
		for _, s := range batch {
			ref := cache[s.lbls.String()]
			var err error
			if s.st != 0 {
				if s.isHistogram() {
					ref, err = app.AppendHistogramSTZeroSample(ref, s.lbls, s.t, s.st, s.h, s.fh)
				} else {
					ref, err = app.AppendSTZeroSample(ref, s.lbls, s.t, s.st)
				}
				require.NoError(t, err)
			}
			if s.isHistogram() {
				ref, err = app.AppendHistogram(ref, s.lbls, s.t, s.h, s.fh)
			} else {
				ref, err = app.Append(ref, s.lbls, s.t, s.v)
			}
			require.NoError(t, err)
			for _, e := range s.exemplars {
				_, err = app.AppendExemplar(ref, s.lbls, e)
				require.NoError(t, err)
			}
			if !s.meta.IsEmpty() {
				_, err = app.UpdateMetadata(ref, s.lbls, s.meta)
				require.NoError(t, err)
			}
			cache[s.lbls.String()] = ref
			refs = append(refs, ref)
		}
		require.NoError(t, app.Commit())
		out = append(out, refs)
	}
	return out
}

// appendBatchesV2 appends batches with one AppenderV2.Append per sample,
// caching the returned ref per series for the next batch.
func appendBatchesV2(t *testing.T, entry storage.AppendableV2, batches [][]paritySample, between func(int)) [][]storage.SeriesRef {
	cache := map[string]storage.SeriesRef{}
	var out [][]storage.SeriesRef
	for i, batch := range batches {
		if between != nil {
			between(i)
		}
		app := entry.AppenderV2(t.Context())
		var refs []storage.SeriesRef
		for _, s := range batch {
			ref, err := app.Append(cache[s.lbls.String()], s.lbls, s.st, s.t, s.v, s.h, s.fh, storage.AppendV2Options{
				Exemplars: s.exemplars,
				Metadata:  s.meta,
			})
			require.NoError(t, err)
			cache[s.lbls.String()] = ref
			refs = append(refs, ref)
		}
		require.NoError(t, app.Commit())
		out = append(out, refs)
	}
	return out
}

func newRecordingStorages(n int) ([]*recordingStorage, []storage.AppendableV2) {
	dests := make([]*recordingStorage, n)
	children := make([]storage.AppendableV2, n)
	for i := range n {
		dests[i] = newRecordingStorage()
		children[i] = dests[i]
	}
	return dests, children
}

// parityEvent is one piece of data written to a recordingStorage.
type parityEvent struct {
	kind  string
	t, st int64
	v     float64
	h     *histogram.Histogram
	fh    *histogram.FloatHistogram
	e     exemplar.Exemplar
	m     metadata.Metadata
}

// recordingStorage is a storage.Storage that behaves like the WAL for the
// purpose of refs: series get a ref when they are first seen, and a ref that
// doesn't match the series' labels is ignored in favour of a lookup by
// labels. Like the WAL, it only implements V1 natively.
type recordingStorage struct {
	storage.Queryable
	storage.ChunkQueryable

	nextRef storage.SeriesRef
	refs    map[string]storage.SeriesRef
	byRef   map[storage.SeriesRef]string
	series  map[string][]parityEvent
}

var _ storage.Storage = (*recordingStorage)(nil)

func newRecordingStorage() *recordingStorage {
	return &recordingStorage{
		nextRef: 100,
		refs:    map[string]storage.SeriesRef{},
		byRef:   map[storage.SeriesRef]string{},
		series:  map[string][]parityEvent{},
	}
}

func (s *recordingStorage) Appender(context.Context) storage.Appender {
	return &recordingStorageAppender{s: s}
}

func (s *recordingStorage) AppenderV2(ctx context.Context) storage.AppenderV2 {
	return adapter.AppenderV1AsV2(s.Appender(ctx))
}

func (s *recordingStorage) StartTime() (int64, error) { return 0, nil }
func (s *recordingStorage) Close() error              { return nil }

// resolve returns the ref for l, allocating one if l is a new series.
func (s *recordingStorage) resolve(ref storage.SeriesRef, l labels.Labels) storage.SeriesRef {
	key := l.String()
	if s.byRef[ref] == key {
		return ref
	}
	if r, ok := s.refs[key]; ok {
		return r
	}
	s.nextRef++
	s.refs[key] = s.nextRef
	s.byRef[s.nextRef] = key
	return s.nextRef
}

func (s *recordingStorage) record(ref storage.SeriesRef, l labels.Labels, ev parityEvent) (storage.SeriesRef, error) {
	ref = s.resolve(ref, l)
	s.series[l.String()] = append(s.series[l.String()], ev)
	return ref, nil
}

type recordingStorageAppender struct{ s *recordingStorage }

func (a *recordingStorageAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	return a.s.record(ref, l, parityEvent{kind: "sample", t: t, v: v})
}

func (a *recordingStorageAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.s.record(ref, l, parityEvent{kind: "histogram", t: t, h: h, fh: fh})
}

func (a *recordingStorageAppender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	return a.s.record(ref, l, parityEvent{kind: "st", t: t, st: st})
}

func (a *recordingStorageAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return a.s.record(ref, l, parityEvent{kind: "histogram_st", t: t, st: st, h: h, fh: fh})
}

func (a *recordingStorageAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return a.s.record(ref, l, parityEvent{kind: "exemplar", e: e})
}

func (a *recordingStorageAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return a.s.record(ref, l, parityEvent{kind: "metadata", m: m})
}

func (a *recordingStorageAppender) SetOptions(*storage.AppendOptions) {}
func (a *recordingStorageAppender) Commit() error                     { return nil }
func (a *recordingStorageAppender) Rollback() error                   { return nil }
