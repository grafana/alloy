package signaltometrics

import (
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"go.opentelemetry.io/collector/config/configoptional"
)

// defaultHistogramBuckets matches the upstream connector's default explicit
// histogram buckets, which are applied when a histogram block is defined
// without any buckets.
var defaultHistogramBuckets = []float64{
	2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
}

// defaultExponentialHistogramMaxSize matches the upstream connector's default
// maximum number of buckets per positive or negative range.
const defaultExponentialHistogramMaxSize = int32(160)

// MetricInfo defines a single metric produced by the connector from a signal.
type MetricInfo struct {
	Name        string `alloy:"name,attr"`
	Description string `alloy:"description,attr,optional"`
	// Unit, if not empty, sets the unit associated with the metric.
	Unit string `alloy:"unit,attr,optional"`
	// IncludeResourceAttributes is a list of resource attributes that need to
	// be included in the generated metric. If the list is empty then all
	// resource attributes are included.
	IncludeResourceAttributes []Attribute `alloy:"include_resource_attributes,block,optional"`
	// Attributes defines the data point attributes to group the generated
	// metric by.
	Attributes []Attribute `alloy:"attributes,block,optional"`
	// Conditions are a set of OTTL conditions which are ORed. Data is
	// processed into metrics only if the sequence evaluates to true.
	Conditions []string `alloy:"conditions,attr,optional"`

	Histogram            *Histogram            `alloy:"histogram,block,optional"`
	ExponentialHistogram *ExponentialHistogram `alloy:"exponential_histogram,block,optional"`
	Sum                  *Sum                  `alloy:"sum,block,optional"`
	Gauge                *Gauge                `alloy:"gauge,block,optional"`
}

func (mi MetricInfo) Convert() config.MetricInfo {
	res := config.MetricInfo{
		Name:                      mi.Name,
		Description:               mi.Description,
		Unit:                      mi.Unit,
		IncludeResourceAttributes: convertAttributes(mi.IncludeResourceAttributes),
		Attributes:                convertAttributes(mi.Attributes),
		Conditions:                mi.Conditions,
		Histogram:                 mi.Histogram.Convert(),
		ExponentialHistogram:      mi.ExponentialHistogram.Convert(),
		Sum:                       mi.Sum.Convert(),
		Gauge:                     mi.Gauge.Convert(),
	}
	return res
}

func convertMetricInfos(infos []MetricInfo) []config.MetricInfo {
	if len(infos) == 0 {
		return nil
	}
	res := make([]config.MetricInfo, 0, len(infos))
	for _, info := range infos {
		res = append(res, info.Convert())
	}
	return res
}

// Attribute defines an attribute used to group or filter the generated metric.
type Attribute struct {
	Key      string `alloy:"key,attr"`
	Optional bool   `alloy:"optional,attr,optional"`
	// DefaultValue is the value used for the attribute if it's missing from the
	// source data. Only one of default_value or optional should be set.
	DefaultValue any `alloy:"default_value,attr,optional"`
}

func (a Attribute) Convert() config.Attribute {
	return config.Attribute{
		Key:          a.Key,
		Optional:     a.Optional,
		DefaultValue: a.DefaultValue,
	}
}

func convertAttributes(attrs []Attribute) []config.Attribute {
	if len(attrs) == 0 {
		return nil
	}
	res := make([]config.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		res = append(res, attr.Convert())
	}
	return res
}

// Histogram defines an explicit bucket histogram metric.
type Histogram struct {
	Buckets []float64 `alloy:"buckets,attr,optional"`
	// Count is an optional OTTL value expression for the histogram count. If
	// empty, each matching record increments the count by one.
	Count string `alloy:"count,attr,optional"`
	// Value is the required OTTL value expression used to record into the
	// histogram.
	Value string `alloy:"value,attr"`
}

var _ syntax.Defaulter = (*Histogram)(nil)

// SetToDefault implements syntax.Defaulter
func (h *Histogram) SetToDefault() {
	h.Buckets = append([]float64{}, defaultHistogramBuckets...)
}

func (h *Histogram) Convert() configoptional.Optional[config.Histogram] {
	if h == nil {
		return configoptional.None[config.Histogram]()
	}

	return configoptional.Some(config.Histogram{
		Buckets: append([]float64{}, h.Buckets...),
		Count:   h.Count,
		Value:   h.Value,
	})
}

// ExponentialHistogram defines a base-2 exponential bucket histogram metric.
type ExponentialHistogram struct {
	MaxSize int32  `alloy:"max_size,attr,optional"`
	Count   string `alloy:"count,attr,optional"`
	Value   string `alloy:"value,attr"`
}

var _ syntax.Defaulter = (*ExponentialHistogram)(nil)

// SetToDefault implements syntax.Defaulter
func (eh *ExponentialHistogram) SetToDefault() {
	eh.MaxSize = defaultExponentialHistogramMaxSize
}

func (eh *ExponentialHistogram) Convert() configoptional.Optional[config.ExponentialHistogram] {
	if eh == nil {
		return configoptional.None[config.ExponentialHistogram]()
	}

	return configoptional.Some(config.ExponentialHistogram{
		MaxSize: eh.MaxSize,
		Count:   eh.Count,
		Value:   eh.Value,
	})
}

// Sum defines a sum metric.
type Sum struct {
	// Value is the required OTTL value expression used to compute the sum.
	Value string `alloy:"value,attr"`
}

func (s *Sum) Convert() configoptional.Optional[config.Sum] {
	if s == nil {
		return configoptional.None[config.Sum]()
	}
	return configoptional.Some(config.Sum{
		Value: s.Value,
	})
}

// Gauge defines a gauge metric.
type Gauge struct {
	// Value is the required OTTL value expression used to compute the gauge.
	Value string `alloy:"value,attr"`
}

func (g *Gauge) Convert() configoptional.Optional[config.Gauge] {
	if g == nil {
		return configoptional.None[config.Gauge]()
	}
	return configoptional.Some(config.Gauge{
		Value: g.Value,
	})
}
