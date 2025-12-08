package echo

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
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

type Arguments struct {
	Format string `alloy:"format,attr,optional"`
}

type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

var DefaultArguments = Arguments{
	Format: "text",
}

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
	c.mut.RLock()
	format := c.args.Format
	c.mut.RUnlock()

	return &echoAppender{
		logger:      c.opts.Logger,
		componentID: c.opts.ID,
		format:      format,
		samples:     make(map[string]sample),
		exemplars:   make(map[string]seriesExemplar),
		histograms:  make(map[string]seriesHistogram),
		metadata:    make(map[string]metadata.Metadata),
	}
}

type echoAppender struct {
	logger      log.Logger
	componentID string
	format      string
	mut         sync.Mutex

	samples    map[string]sample
	exemplars  map[string]seriesExemplar
	histograms map[string]seriesHistogram
	metadata   map[string]metadata.Metadata
}

type sample struct {
	Labels         labels.Labels
	Timestamp      int64
	Value          float64
	PrintTimestamp bool
}

type seriesExemplar struct {
	Labels   labels.Labels
	Exemplar exemplar.Exemplar
}

type seriesHistogram struct {
	Labels    labels.Labels
	Histogram histogram.Histogram
}

func (a *echoAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	key := l.String()
	a.samples[key] = sample{
		Labels:         l.Copy(),
		Timestamp:      t,
		Value:          v,
		PrintTimestamp: t > 0,
	}

	if ref == 0 {
		ref = storage.SeriesRef(len(a.samples))
	}
	return ref, nil
}

func (a *echoAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	key := l.String()
	a.exemplars[key] = seriesExemplar{
		Labels:   l.Copy(),
		Exemplar: e,
	}

	if ref == 0 {
		ref = storage.SeriesRef(1)
	}
	return ref, nil
}

func (a *echoAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	key := l.String()
	if h != nil {
		a.histograms[key] = seriesHistogram{
			Labels:    l.Copy(),
			Histogram: *h,
		}
	}

	if ref == 0 {
		ref = storage.SeriesRef(len(a.histograms))
	}
	return ref, nil
}

func (a *echoAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	metricName := l.Get(model.MetricNameLabel)
	if metricName != "" {
		a.metadata[metricName] = m
	}

	if ref == 0 {
		ref = storage.SeriesRef(1)
	}
	return ref, nil
}

func (a *echoAppender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	key := l.String()
	a.samples[key] = sample{
		Labels:         l.Copy(),
		Timestamp:      t,
		Value:          0,
		PrintTimestamp: t > 0,
	}

	if ref == 0 {
		ref = storage.SeriesRef(len(a.samples))
	}
	return ref, nil
}

func (a *echoAppender) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	a.mut.Lock()
	defer a.mut.Unlock()

	key := l.String()
	if h != nil {
		a.histograms[key] = seriesHistogram{
			Labels:    l.Copy(),
			Histogram: *h,
		}
	}

	if ref == 0 {
		ref = storage.SeriesRef(len(a.histograms))
	}
	return ref, nil
}

func (a *echoAppender) Commit() error {
	a.mut.Lock()
	defer a.mut.Unlock()

	families := a.buildMetricFamilies()

	var buf bytes.Buffer
	var expFormat expfmt.Format

	switch a.format {
	case "openmetrics":
		expFormat = expfmt.NewFormat(expfmt.TypeOpenMetrics)
	case "text", "":
		expFormat = expfmt.NewFormat(expfmt.TypeTextPlain)
	default:
		level.Warn(a.logger).Log("component", a.componentID, "msg", "unknown format, using text", "format", a.format)
		expFormat = expfmt.NewFormat(expfmt.TypeTextPlain)
	}

	encoder := expfmt.NewEncoder(&buf, expFormat)

	for _, family := range families {
		if err := encoder.Encode(family); err != nil {
			level.Error(a.logger).Log("component", a.componentID, "error", "failed to encode metric family", "family", family.GetName(), "err", err)
			continue
		}
	}

	level.Info(a.logger).Log("component", a.componentID, "metrics", buf.String())

	a.clearStorage()

	return nil
}

func (a *echoAppender) Rollback() error {
	a.mut.Lock()
	defer a.mut.Unlock()

	a.clearStorage()

	return nil
}

func (a *echoAppender) SetOptions(opts *storage.AppendOptions) {
}

func (a *echoAppender) clearStorage() {
	for k := range a.samples {
		delete(a.samples, k)
	}
	for k := range a.exemplars {
		delete(a.exemplars, k)
	}
	for k := range a.histograms {
		delete(a.histograms, k)
	}
	for k := range a.metadata {
		delete(a.metadata, k)
	}
}

func (a *echoAppender) buildMetricFamilies() []*dto.MetricFamily {
	b := builder{
		samples:    a.samples,
		exemplars:  a.exemplars,
		metadata:   a.metadata,
		histograms: a.histograms,

		familyLookup: make(map[string]*dto.MetricFamily),
	}
	return b.build()
}

type builder struct {
	samples    map[string]sample
	exemplars  map[string]seriesExemplar
	metadata   map[string]metadata.Metadata
	histograms map[string]seriesHistogram

	families     []*dto.MetricFamily
	familyLookup map[string]*dto.MetricFamily
}

func (b *builder) build() []*dto.MetricFamily {
	b.buildFamiliesFromMetadata()
	b.buildMetricsFromSamples()
	b.buildHistograms()
	b.assignExemplars()
	b.sortFamilies()

	return b.families
}

func (b *builder) buildFamiliesFromMetadata() {
	for metricName, md := range b.metadata {
		family := &dto.MetricFamily{
			Name: ptr.To(metricName),
			Help: ptr.To(md.Help),
		}

		family.Type = ptr.To(dto.MetricType_UNTYPED)

		b.families = append(b.families, family)
		b.familyLookup[metricName] = family
	}
}

func (b *builder) buildMetricsFromSamples() {
	for _, sample := range b.samples {
		metricName := sample.Labels.Get(model.MetricNameLabel)
		if metricName == "" {
			continue
		}

		family := b.getOrCreateFamily(metricName)
		metric := &dto.Metric{}

		sample.Labels.Range(func(label labels.Label) {
			if label.Name != model.MetricNameLabel {
				metric.Label = append(metric.Label, &dto.LabelPair{
					Name:  ptr.To(string(label.Name)),
					Value: ptr.To(string(label.Value)),
				})
			}
		})

		switch family.GetType() {
		case dto.MetricType_COUNTER:
			metric.Counter = &dto.Counter{Value: ptr.To(sample.Value)}
		case dto.MetricType_GAUGE:
			metric.Gauge = &dto.Gauge{Value: ptr.To(sample.Value)}
		default:
			metric.Untyped = &dto.Untyped{Value: ptr.To(sample.Value)}
		}

		if sample.PrintTimestamp {
			metric.TimestampMs = ptr.To(sample.Timestamp)
		}

		family.Metric = append(family.Metric, metric)
	}
}

func (b *builder) buildHistograms() {
	for _, hist := range b.histograms {
		metricName := hist.Labels.Get(model.MetricNameLabel)
		if metricName == "" {
			continue
		}

		family := b.getOrCreateFamily(metricName)
		family.Type = ptr.To(dto.MetricType_HISTOGRAM)

		metric := &dto.Metric{}

		hist.Labels.Range(func(label labels.Label) {
			if label.Name != model.MetricNameLabel {
				metric.Label = append(metric.Label, &dto.LabelPair{
					Name:  ptr.To(string(label.Name)),
					Value: ptr.To(string(label.Value)),
				})
			}
		})

		metric.Histogram = &dto.Histogram{
			SampleCount: ptr.To(hist.Histogram.Count),
			SampleSum:   ptr.To(hist.Histogram.Sum),
		}

		family.Metric = append(family.Metric, metric)
	}
}

func (b *builder) assignExemplars() {
	for _, ex := range b.exemplars {
		metricName := ex.Labels.Get(model.MetricNameLabel)
		if metricName == "" {
			continue
		}

		family := b.familyLookup[metricName]
		if family == nil {
			continue
		}

		for _, metric := range family.Metric {
			if b.labelsMatch(ex.Labels, metric.Label) {
				exemplar := &dto.Exemplar{
					Value: ptr.To(ex.Exemplar.Value),
				}

				if ex.Exemplar.HasTs {
					ts := ex.Exemplar.Ts / 1000
					exemplar.Timestamp = timestamppb.New(time.Unix(ts, (ex.Exemplar.Ts%1000)*1e6))
				}

				ex.Exemplar.Labels.Range(func(label labels.Label) {
					exemplar.Label = append(exemplar.Label, &dto.LabelPair{
						Name:  ptr.To(string(label.Name)),
						Value: ptr.To(string(label.Value)),
					})
				})

				if metric.Counter != nil {
					metric.Counter.Exemplar = exemplar
				} else if metric.Histogram != nil {
					metric.Histogram.Bucket = append(metric.Histogram.Bucket, &dto.Bucket{
						Exemplar: exemplar,
					})
				}
				break
			}
		}
	}
}

func (b *builder) getOrCreateFamily(metricName string) *dto.MetricFamily {
	if family, exists := b.familyLookup[metricName]; exists {
		return family
	}

	family := &dto.MetricFamily{
		Name: ptr.To(metricName),
		Type: ptr.To(dto.MetricType_UNTYPED),
	}

	b.families = append(b.families, family)
	b.familyLookup[metricName] = family
	return family
}

func (b *builder) labelsMatch(seriesLabels labels.Labels, metricLabels []*dto.LabelPair) bool {
	if seriesLabels.Len() != len(metricLabels)+1 {
		return false
	}

	foundErr := seriesLabels.Validate(func(seriesLabel labels.Label) error {
		if seriesLabel.Name == model.MetricNameLabel {
			return nil
		}

		for _, metricLabel := range metricLabels {
			if string(seriesLabel.Name) == metricLabel.GetName() && string(seriesLabel.Value) == metricLabel.GetValue() {
				return nil
			}
		}

		return errors.New("label not found")
	})

	return foundErr == nil
}

func (b *builder) sortFamilies() {
	sort.Slice(b.families, func(i, j int) bool {
		return b.families[i].GetName() < b.families[j].GetName()
	})

	for _, family := range b.families {
		sort.Slice(family.Metric, func(i, j int) bool {
			return b.compareMetrics(family.Metric[i], family.Metric[j])
		})
	}
}

func (b *builder) compareMetrics(a, bb *dto.Metric) bool {
	aLabels := make([]string, 0, len(a.Label))
	bLabels := make([]string, 0, len(bb.Label))

	for _, label := range a.Label {
		aLabels = append(aLabels, label.GetName()+"="+label.GetValue())
	}
	for _, label := range bb.Label {
		bLabels = append(bLabels, label.GetName()+"="+label.GetValue())
	}

	sort.Strings(aLabels)
	sort.Strings(bLabels)

	for i := 0; i < len(aLabels) && i < len(bLabels); i++ {
		if aLabels[i] != bLabels[i] {
			return aLabels[i] < bLabels[i]
		}
	}

	return len(aLabels) < len(bLabels)
}
