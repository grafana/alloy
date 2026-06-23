package metadata

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Metadata struct {
	Name         string            `yaml:"name"`
	Platforms    []Platform        `yaml:"platforms"`
	Requirements []Requirement     `yaml:"requirements"`
	Namespace    string            `yaml:"namespace"`
	Subsystem    string            `yaml:"subsystem"`
	Metrics      map[string]Metric `yaml:"metrics"`
}

type Requirement struct {
	Description string `yaml:"description"`
	Reference   string `yaml:"reference"`
}

type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformDarwin  Platform = "darwin"
	PlatformFreeBSD Platform = "freebsd"
)

var validPlatforms = map[Platform]bool{
	PlatformLinux:   true,
	PlatformWindows: true,
	PlatformDarwin:  true,
	PlatformFreeBSD: true,
}

func (p *Platform) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	platform := Platform(s)
	if !validPlatforms[platform] {
		return fmt.Errorf("line %d: invalid platform %q (must be one of: linux, windows, darwin, freebsd)", value.Line, s)
	}
	*p = platform
	return nil
}

// Metric describes a single Prometheus metric emitted by a component.
type Metric struct {
	Type   MetricType `yaml:"type"`
	Help   string     `yaml:"help"`
	Labels []string   `yaml:"labels"`
}

// GoType returns the struct field type for the metric.
func (m Metric) GoType() string { return metricKinds[m.Type].GoType }

// NewFunc returns the client_golang constructor for the metric.
func (m Metric) NewFunc() string { return metricKinds[m.Type].NewFunc }

// OptsType returns the client_golang opts type for the metric.
func (m Metric) OptsType() string { return metricKinds[m.Type].OptsType }

// IsVec reports whether the metric constructor takes a label slice.
func (m Metric) IsVec() bool { return metricKinds[m.Type].IsVec }

// PromType returns the lowercase base Prometheus type used in documentation,
// e.g. "CounterVec" becomes "counter".
func (m Metric) PromType() string {
	return strings.ToLower(strings.TrimSuffix(string(m.Type), "Vec"))
}

// MetricType is the kind of Prometheus metric, matching the client_golang
// constructor names.
type MetricType string

const (
	MetricCounter      MetricType = "Counter"
	MetricCounterVec   MetricType = "CounterVec"
	MetricGauge        MetricType = "Gauge"
	MetricGaugeVec     MetricType = "GaugeVec"
	MetricHistogram    MetricType = "Histogram"
	MetricHistogramVec MetricType = "HistogramVec"
	MetricSummary      MetricType = "Summary"
	MetricSummaryVec   MetricType = "SummaryVec"
)

func (t *MetricType) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	mt := MetricType(s)
	if _, ok := metricKinds[mt]; !ok {
		return fmt.Errorf("line %d: invalid metric type %q", value.Line, s)
	}
	*t = mt
	return nil
}

// metricKind holds the code-generation details for a metric type.
type metricKind struct {
	GoType   string
	NewFunc  string
	OptsType string
	IsVec    bool
}

var metricKinds = map[MetricType]metricKind{
	MetricCounter:      {"prometheus.Counter", "prometheus.NewCounter", "prometheus.CounterOpts", false},
	MetricCounterVec:   {"*prometheus.CounterVec", "prometheus.NewCounterVec", "prometheus.CounterOpts", true},
	MetricGauge:        {"prometheus.Gauge", "prometheus.NewGauge", "prometheus.GaugeOpts", false},
	MetricGaugeVec:     {"*prometheus.GaugeVec", "prometheus.NewGaugeVec", "prometheus.GaugeOpts", true},
	MetricHistogram:    {"prometheus.Histogram", "prometheus.NewHistogram", "prometheus.HistogramOpts", false},
	MetricHistogramVec: {"*prometheus.HistogramVec", "prometheus.NewHistogramVec", "prometheus.HistogramOpts", true},
	MetricSummary:      {"prometheus.Summary", "prometheus.NewSummary", "prometheus.SummaryOpts", false},
	MetricSummaryVec:   {"*prometheus.SummaryVec", "prometheus.NewSummaryVec", "prometheus.SummaryOpts", true},
}
