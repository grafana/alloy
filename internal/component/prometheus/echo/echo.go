package echo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.echo",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct{}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

var DefaultArguments = Arguments{}

func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

var (
	_ component.Component = (*Component)(nil)
	_ storage.Appendable  = (*Component)(nil)
)

type Component struct {
	opts component.Options
	mut  sync.RWMutex
	args Arguments
}

func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: o,
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	if o.OnStateChange != nil {
		o.OnStateChange(Exports{Receiver: c})
	}
	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs
	return nil
}

func (c *Component) Appender(ctx context.Context) storage.Appender {
	return &echoAppender{
		logger:      c.opts.Logger,
		componentID: c.opts.ID,
	}
}

type echoAppender struct {
	logger      log.Logger
	componentID string
	metrics     []metricData
	mut         sync.Mutex
}

type metricData struct {
	labels       labels.Labels
	timestamp    int64
	value        float64
	histogram    *histogram.Histogram
	fHistogram   *histogram.FloatHistogram
	exemplar     *exemplar.Exemplar
	metadata     *metadata.Metadata
	isZeroSample bool
	ct           int64
}

func (a *echoAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()
	a.metrics = append(a.metrics, metricData{
		labels:    l.Copy(),
		timestamp: t,
		value:     v,
	})
	if ref == 0 {
		ref = storage.SeriesRef(len(a.metrics))
	}
	return ref, nil
}

func (a *echoAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()
	for i := len(a.metrics) - 1; i >= 0; i-- {
		if labels.Equal(a.metrics[i].labels, l) {
			a.metrics[i].exemplar = &e
			break
		}
	}
	if ref == 0 {
		ref = storage.SeriesRef(1)
	}
	return ref, nil
}

func (a *echoAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()
	a.metrics = append(a.metrics, metricData{
		labels:     l.Copy(),
		timestamp:  t,
		histogram:  h,
		fHistogram: fh,
	})
	if ref == 0 {
		ref = storage.SeriesRef(len(a.metrics))
	}
	return ref, nil
}

func (a *echoAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()
	for i := range a.metrics {
		if labels.Equal(a.metrics[i].labels, l) {
			a.metrics[i].metadata = &m
		}
	}
	if ref == 0 {
		ref = storage.SeriesRef(1)
	}
	return ref, nil
}

func (a *echoAppender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()
	a.metrics = append(a.metrics, metricData{
		labels:       l.Copy(),
		timestamp:    t,
		value:        0,
		isZeroSample: true,
		ct:           ct,
	})
	if ref == 0 {
		ref = storage.SeriesRef(len(a.metrics))
	}
	return ref, nil
}

func (a *echoAppender) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()
	a.metrics = append(a.metrics, metricData{
		labels:       l.Copy(),
		timestamp:    t,
		histogram:    h,
		fHistogram:   fh,
		isZeroSample: true,
		ct:           ct,
	})
	if ref == 0 {
		ref = storage.SeriesRef(len(a.metrics))
	}
	return ref, nil
}

func (a *echoAppender) Commit() error {
	a.mut.Lock()
	defer a.mut.Unlock()
	metricGroups := make(map[string][]metricData)
	for _, m := range a.metrics {
		name := m.labels.Get(model.MetricNameLabel)
		if name == "" {
			name = "{no_metric_name}"
		}
		metricGroups[name] = append(metricGroups[name], m)
	}
	var output strings.Builder
	output.WriteString(fmt.Sprintf("# Prometheus metrics received by %s\n", a.componentID))
	output.WriteString(fmt.Sprintf("# Timestamp: %s\n", time.Now().Format(time.RFC3339)))
	for metricName, metrics := range metricGroups {
		if len(metrics) == 0 {
			continue
		}
		if metrics[0].metadata != nil {
			if metrics[0].metadata.Help != "" {
				output.WriteString(fmt.Sprintf("# HELP %s %s\n", metricName, metrics[0].metadata.Help))
			}
			if metrics[0].metadata.Type != "" {
				output.WriteString(fmt.Sprintf("# TYPE %s %s\n", metricName, metrics[0].metadata.Type))
			}
		}
		for _, m := range metrics {
			output.WriteString(formatMetric(m))
		}
	}
	level.Info(a.logger).Log("component", a.componentID, "metrics", output.String())
	a.metrics = a.metrics[:0]
	return nil
}

func (a *echoAppender) Rollback() error {
	a.mut.Lock()
	defer a.mut.Unlock()
	a.metrics = a.metrics[:0]
	return nil
}

func formatMetric(m metricData) string {
	var output strings.Builder
	metricName := m.labels.Get(model.MetricNameLabel)
	if metricName == "" {
		metricName = "{no_metric_name}"
	}
	output.WriteString(metricName)
	labelPairs := make([]string, 0, len(m.labels)-1)
	for _, l := range m.labels {
		if l.Name != model.MetricNameLabel {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%q", l.Name, l.Value))
		}
	}
	if len(labelPairs) > 0 {
		output.WriteString("{")
		output.WriteString(strings.Join(labelPairs, ","))
		output.WriteString("}")
	}
	if m.histogram != nil || m.fHistogram != nil {
		output.WriteString(" {histogram}")
	} else {
		output.WriteString(fmt.Sprintf(" %g", m.value))
	}
	if m.timestamp > 0 {
		output.WriteString(fmt.Sprintf(" %d", m.timestamp))
	}
	if m.exemplar != nil {
		output.WriteString(fmt.Sprintf(" # {%s} %g", m.exemplar.Labels, m.exemplar.Value))
	}
	if m.isZeroSample && m.ct > 0 {
		output.WriteString(fmt.Sprintf(" # CreatedTimestamp: %d", m.ct))
	}
	output.WriteString("\n")
	return output.String()
}

func (a *echoAppender) SetOptions(opts *storage.AppendOptions) {
}
