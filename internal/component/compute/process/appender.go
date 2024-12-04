package process

import (
	"context"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"time"
)

var _ storage.Appender = (*bulkAppender)(nil)
var _ storage.Appendable = (*bulkAppendable)(nil)

type bulkAppendable struct {
	wasm                       *WasmPlugin
	metrics                    []*PrometheusMetric
	next                       storage.Appendable
	timeMetric                 prom.Counter
	prometheusRecordsProcessed prom.Counter
	bridge                     *bridge
}

func (b *bulkAppendable) Appender(ctx context.Context) storage.Appender {
	return &bulkAppender{
		ctx:                        ctx,
		wasm:                       b.wasm,
		timeMetric:                 b.timeMetric,
		prometheusRecordsProcessed: b.prometheusRecordsProcessed,
		bridge:                     b.bridge,
	}
}

func (b *bulkAppendable) send(metrics []*PrometheusMetric) {
	app := b.next.Appender(context.TODO())
	for _, m := range metrics {
		labelsBack := make(labels.Labels, len(m.Labels))
		for i, l := range m.Labels {
			labelsBack[i] = labels.Label{
				Name:  l.Name,
				Value: l.Value,
			}
		}
		// We explicitly dont care about errors from append
		_, _ = app.Append(0, labelsBack, m.Timestampms, m.Value)
	}
	// We explicitly dont care about errors from commit
	_ = app.Commit()
}

type bulkAppender struct {
	ctx                        context.Context
	wasm                       *WasmPlugin
	metrics                    []*PrometheusMetric
	timeMetric                 prom.Counter
	prometheusRecordsProcessed prom.Counter
	bridge                     *bridge
}

func (b *bulkAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	labels := make([]*Label, len(l))
	for j, lbl := range l {
		labels[j] = &Label{
			Name:  lbl.Name,
			Value: lbl.Value,
		}
	}
	m := &PrometheusMetric{
		Value:       v,
		Timestampms: t,
		Labels:      labels,
	}
	b.metrics = append(b.metrics, m)
	return ref, nil
}

func (b *bulkAppender) Commit() error {
	return b.process()
}

func (b *bulkAppender) Rollback() error {
	return nil
}

func (b *bulkAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) process() error {
	pt := &Passthrough{
		// TODO reduce the number of random types that
		// represent the same thing.
		Prommetrics: b.metrics,
	}
	b.prometheusRecordsProcessed.Add(float64(len(b.metrics)))
	start := time.Now()
	outpt, err := b.wasm.Process(pt)
	elapsed := time.Since(start)
	b.timeMetric.Add(float64(elapsed.Milliseconds()))
	if err != nil {
		return err
	}
	// Passthrough may have generated other types so we need to pass it to the bridge to handle.
	b.bridge.sendPassthrough(outpt)
	return nil
}
