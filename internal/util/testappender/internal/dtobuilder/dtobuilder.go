package dtobuilder

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/textparse"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
)

// Sample represents an individually written sample to a storage.Appender.
type Sample struct {
	Labels         labels.Labels
	Timestamp      int64
	Value          float64
	PrintTimestamp bool
}

// SeriesExemplar represents an individually written exemplar to a
// storage.Appender.
type SeriesExemplar struct {
	// Labels is the labels of the series exposing the exemplar, not the labels
	// on the exemplar itself.
	Labels   labels.Labels
	Exemplar exemplar.Exemplar
}

type SeriesHistogram struct {
	Labels    labels.Labels
	Histogram histogram.Histogram
}

// Build converts a series of written samples, exemplars, and metadata into a
// slice of *dto.MetricFamily.
func Build(
	samples map[string]Sample,
	exemplars map[string]SeriesExemplar,
	histograms map[string]SeriesHistogram,
	metadata map[string]metadata.Metadata,
) []*dto.MetricFamily {

	b := builder{
		Samples:    samples,
		Exemplars:  exemplars,
		Metadata:   metadata,
		Histograms: histograms,

		familyLookup: make(map[string]*dto.MetricFamily),
	}
	return b.Build()
}

type builder struct {
	Samples    map[string]Sample
	Exemplars  map[string]SeriesExemplar
	Metadata   map[string]metadata.Metadata
	Histograms map[string]SeriesHistogram

	families     []*dto.MetricFamily
	familyLookup map[string]*dto.MetricFamily
}

// Build converts the dtoBuilder's Samples, Exemplars, and Metadata into a set
// of []*dto.MetricFamily.
func (b *builder) Build() []*dto.MetricFamily {
	// *dto.MetricFamily represents a set of samples for a given family of
	// metrics. All metrics with the same __name__ belong to the same family.
	//
	// Each *dto.MetricFamily has a set of *dto.Metric, which contain individual
	// samples within that family. The *dto.Metric is where non-__name__ labels
	// are kept.
	//
	// *dto.Metrics can represent counters, gauges, summaries, histograms, and
	// untyped values.
	//
	// In the case of a summary, the *dto.Metric contains multiple samples,
	// holding each quantile, the _count, and the _sum. Similarly for histograms,
	// the *dto.Metric contains each bucket, the _count, and the _sum.
	//
	// Because *dto.Metrics for summaries and histograms contain multiple
	// samples, Build must roll up individually recorded samples into the
	// appropriate *dto.Metric. See buildMetricsFromSamples for more information.

	// We *must* do things in the following order:
	//
	// 1. Populate the families from metadata so we know what fields in
	//    *dto.Metric to set.
	// 2. Populate *dto.Metric values from provided samples.
	// 3. Build Histograms (used for Native Histograms).
	// 4. Assign exemplars to *dto.Metrics as appropriate.
	b.buildFamiliesFromMetadata()
	b.buildMetricsFromSamples()
	b.buildHistograms()
	b.injectExemplars()

	// Sort all the data before returning.
	sortMetricFamilies(b.families)
	return b.families
}

// buildFamiliesFromMetadata populates the list of families based on the
// metadata known to the dtoBuilder. familyLookup will be updated for all
// metrics which map to the same family.
//
// In the case of summaries and histograms, multiple metrics map to the same
// family (the bucket/quantile, the _sum, and the _count metrics).
func (b *builder) buildFamiliesFromMetadata() {
	for familyName, m := range b.Metadata {
		mt := textParseToMetricType(m.Type)
		mf := &dto.MetricFamily{
			Name: ptr.To(familyName),
			Type: &mt,
		}
		if m.Help != "" {
			mf.Help = ptr.To(m.Help)
		}

		b.families = append(b.families, mf)

		// Determine how to populate the lookup table.
		switch mt {
		case dto.MetricType_SUMMARY:
			// Summaries include metrics with the family name (for quantiles),
			// followed by _sum and _count suffixes.
			b.familyLookup[familyName] = mf
			b.familyLookup[familyName+"_sum"] = mf
			b.familyLookup[familyName+"_count"] = mf
		case dto.MetricType_HISTOGRAM:
			// Metadata types do not differentiate between histogram and exponential histogram yet.
			// This is a temporary hacky way which allow us to test exponential histogram by having exponential in the name.
			if strings.Contains(familyName, "exponential") {
				b.familyLookup[familyName] = mf
				break
			}
			// Histograms include metrics for _bucket, _sum, and _count suffixes.
			b.familyLookup[familyName+"_bucket"] = mf
			b.familyLookup[familyName+"_sum"] = mf
			b.familyLookup[familyName+"_count"] = mf
		default:
			// Everything else matches the family name exactly.
			b.familyLookup[familyName] = mf
		}
	}
}

func textParseToMetricType(tp textparse.MetricType) dto.MetricType {
	switch tp {
	case textparse.MetricTypeCounter:
		return dto.MetricType_COUNTER
	case textparse.MetricTypeGauge:
		return dto.MetricType_GAUGE
	case textparse.MetricTypeHistogram:
		return dto.MetricType_HISTOGRAM
	case textparse.MetricTypeSummary:
		return dto.MetricType_SUMMARY
	default:
		// There are other values for m.Type, but they're all
		// OpenMetrics-specific and we're only converting into the Prometheus
		// exposition format.
		return dto.MetricType_UNTYPED
	}
}

func (b *builder) buildHistograms() {
	for _, histogram := range b.Histograms {
		metricName := histogram.Labels.Get(model.MetricNameLabel)
		mf := b.getOrCreateMetricFamily(metricName)

		m := getOrCreateMetric(mf, histogram.Labels)
		sum := histogram.Histogram.Sum
		count := histogram.Histogram.Count
		schema := histogram.Histogram.Schema
		zeroThreshold := histogram.Histogram.ZeroThreshold
		zeroCount := histogram.Histogram.ZeroCount

		m.Histogram = &dto.Histogram{
			PositiveSpan:  convertSpans(histogram.Histogram.PositiveSpans),
			NegativeSpan:  convertSpans(histogram.Histogram.NegativeSpans),
			PositiveDelta: histogram.Histogram.PositiveBuckets,
			NegativeDelta: histogram.Histogram.NegativeBuckets,
			SampleSum:     &sum,
			SampleCount:   &count,
			Schema:        &schema,
			ZeroThreshold: &zeroThreshold,
			ZeroCount:     &zeroCount,
		}
	}
}

func convertSpans(spans []histogram.Span) []*dto.BucketSpan {
	bucketSpan := make([]*dto.BucketSpan, len(spans))
	for i, span := range spans {
		bucketSpan[i] = convertSpan(span)
	}
	return bucketSpan
}

func convertSpan(span histogram.Span) *dto.BucketSpan {
	offset := span.Offset
	length := span.Length
	return &dto.BucketSpan{
		Offset: &offset,
		Length: &length,
	}
}

// buildMetricsFromSamples populates *dto.Metrics. If the MetricFamily doesn't
// exist for a given sample, a new one is created.
func (b *builder) buildMetricsFromSamples() {
	for _, sample := range b.Samples {
		// Get or create the metric family.
		metricName := sample.Labels.Get(model.MetricNameLabel)
		mf := b.getOrCreateMetricFamily(metricName)

		// Retrieve the *dto.Metric based on labels.
		m := getOrCreateMetric(mf, sample.Labels)
		if sample.PrintTimestamp {
			m.TimestampMs = ptr.To(sample.Timestamp)
		}

		switch familyType(mf) {
		case dto.MetricType_COUNTER:
			m.Counter = &dto.Counter{
				Value: ptr.To(sample.Value),
			}

		case dto.MetricType_GAUGE:
			m.Gauge = &dto.Gauge{
				Value: ptr.To(sample.Value),
			}

		case dto.MetricType_SUMMARY:
			if m.Summary == nil {
				m.Summary = &dto.Summary{}
			}

			switch {
			case metricName == mf.GetName()+"_count":
				val := uint64(sample.Value)
				m.Summary.SampleCount = &val
			case metricName == mf.GetName()+"_sum":
				m.Summary.SampleSum = ptr.To(sample.Value)
			case metricName == mf.GetName():
				quantile, err := strconv.ParseFloat(sample.Labels.Get(model.QuantileLabel), 64)
				if err != nil {
					continue
				}

				m.Summary.Quantile = append(m.Summary.Quantile, &dto.Quantile{
					Quantile: &quantile,
					Value:    ptr.To(sample.Value),
				})
			}

		case dto.MetricType_UNTYPED:
			m.Untyped = &dto.Untyped{
				Value: ptr.To(sample.Value),
			}

		case dto.MetricType_HISTOGRAM:
			if m.Histogram == nil {
				m.Histogram = &dto.Histogram{}
			}

			switch {
			case metricName == mf.GetName()+"_count":
				val := uint64(sample.Value)
				m.Histogram.SampleCount = &val
			case metricName == mf.GetName()+"_sum":
				m.Histogram.SampleSum = ptr.To(sample.Value)
			case metricName == mf.GetName()+"_bucket":
				boundary, err := strconv.ParseFloat(sample.Labels.Get(model.BucketLabel), 64)
				if err != nil {
					continue
				}

				count := uint64(sample.Value)

				m.Histogram.Bucket = append(m.Histogram.Bucket, &dto.Bucket{
					UpperBound:      &boundary,
					CumulativeCount: &count,
				})
			}
		}
	}
}

func (b *builder) getOrCreateMetricFamily(familyName string) *dto.MetricFamily {
	mf, ok := b.familyLookup[familyName]
	if ok {
		return mf
	}

	mt := dto.MetricType_UNTYPED
	mf = &dto.MetricFamily{
		Name: &familyName,
		Type: &mt,
	}
	b.families = append(b.families, mf)
	b.familyLookup[familyName] = mf
	return mf
}

func getOrCreateMetric(mf *dto.MetricFamily, l labels.Labels) *dto.Metric {
	metricLabels := toLabelPairs(familyType(mf), l)

	for _, check := range mf.Metric {
		if labelPairsEqual(check.Label, metricLabels) {
			return check
		}
	}

	m := &dto.Metric{
		Label: metricLabels,
	}
	mf.Metric = append(mf.Metric, m)
	return m
}

// toLabelPairs converts labels.Labels into []*dto.LabelPair. The __name__
// label is always dropped, since the metric name is retrieved from the family
// name instead.
//
// The quantile label is dropped for summaries, and the le label is dropped for
// histograms.
func toLabelPairs(mt dto.MetricType, ls labels.Labels) []*dto.LabelPair {
	res := make([]*dto.LabelPair, 0, len(ls))
	for _, l := range ls {
		if l.Name == model.MetricNameLabel {
			continue
		} else if l.Name == model.QuantileLabel && mt == dto.MetricType_SUMMARY {
			continue
		} else if l.Name == model.BucketLabel && mt == dto.MetricType_HISTOGRAM {
			continue
		}

		res = append(res, &dto.LabelPair{
			Name:  ptr.To(l.Name),
			Value: ptr.To(l.Value),
		})
	}

	sort.Slice(res, func(i, j int) bool {
		switch {
		case *res[i].Name < *res[j].Name:
			return true
		case *res[i].Value < *res[j].Value:
			return true
		default:
			return false
		}
	})
	return res
}

func labelPairsEqual(a, b []*dto.LabelPair) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if *a[i].Name != *b[i].Name || *a[i].Value != *b[i].Value {
			return false
		}
	}

	return true
}

func familyType(mf *dto.MetricFamily) dto.MetricType {
	ty := mf.Type
	if ty == nil {
		return dto.MetricType_UNTYPED
	}
	return *ty
}

// injectExemplars populates the exemplars in the various *dto.Metric
// instances. Exemplars are ignored if the parent *dto.MetricFamily doesn't
// support exeplars based on metric type.
func (b *builder) injectExemplars() {
	for _, e := range b.Exemplars {
		// Get or create the metric family.
		exemplarName := e.Labels.Get(model.MetricNameLabel)

		mf, ok := b.familyLookup[exemplarName]
		if !ok {
			// No metric family, which means no corresponding sample; ignore.
			continue
		}

		m := getMetric(mf, e.Labels)
		if m == nil {
			continue
		}

		// Only counters and histograms support exemplars.
		switch familyType(mf) {
		case dto.MetricType_COUNTER:
			if m.Counter == nil {
				// Sample never added; ignore.
				continue
			}
			m.Counter.Exemplar = convertExemplar(dto.MetricType_COUNTER, e.Exemplar)
		case dto.MetricType_HISTOGRAM:
			if m.Histogram == nil {
				// Sample never added; ignore.
				continue
			}

			switch {
			case exemplarName == mf.GetName()+"_bucket":
				boundary, err := strconv.ParseFloat(e.Labels.Get(model.BucketLabel), 64)
				if err != nil {
					continue
				}
				bucket := findBucket(m.Histogram, boundary)
				if bucket == nil {
					continue
				}
				bucket.Exemplar = convertExemplar(dto.MetricType_HISTOGRAM, e.Exemplar)
			// Exemplars support for native histograms is not yet available: https://github.com/prometheus/client_golang/issues/1126
			// We need to add the exemplars in the classic histogram buckets
			case exemplarName == mf.GetName():
				m.Histogram.Bucket = append(m.Histogram.Bucket, &dto.Bucket{
					Exemplar: convertExemplar(dto.MetricType_HISTOGRAM, e.Exemplar),
				})
			}
		}
	}
}

func getMetric(mf *dto.MetricFamily, l labels.Labels) *dto.Metric {
	metricLabels := toLabelPairs(familyType(mf), l)

	for _, check := range mf.Metric {
		if labelPairsEqual(check.Label, metricLabels) {
			return check
		}
	}

	return nil
}

func convertExemplar(mt dto.MetricType, e exemplar.Exemplar) *dto.Exemplar {
	res := &dto.Exemplar{
		Label: toLabelPairs(mt, e.Labels),
		Value: &e.Value,
	}
	if e.HasTs {
		res.Timestamp = timestamppb.New(time.UnixMilli(e.Ts))
	}
	return res
}

func findBucket(h *dto.Histogram, bound float64) *dto.Bucket {
	for _, b := range h.GetBucket() {
		// Special handling because Inf - Inf returns NaN.
		if bound == math.Inf(1) && b.GetUpperBound() == math.Inf(1) {
			return b
		}

		// If it's close enough, use the bucket.
		if math.Abs(b.GetUpperBound()-bound) < 1e-9 {
			return b
		}
	}

	return nil
}
