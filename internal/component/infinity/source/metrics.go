package source

import (
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

type sample struct {
	labels labels.Labels
	value  float64
}

type metricsResult struct {
	samples    []sample
	duplicates int
	// overridden is true when a column label was replaced by job or instance.
	overridden bool
	// reservedUp is true when a column's final metric name is up. That name
	// is reserved for the synthetic up sample, so the column's sample is
	// dropped.
	reservedUp bool
}

// upMetricName is the metric name of the synthetic up sample. A data column
// with this final name, after prefix and sanitizing, is dropped instead of
// sent, so it cannot collide with the synthetic sample.
const upMetricName = "up"

// frameToSamples maps each row to current-state samples. String columns
// become labels. Numeric and boolean columns become samples. Time columns
// are ignored, because samples use the poll time.
func frameToSamples(f *data.Frame, job, instance string, m metricsSpec) (metricsResult, error) {
	var res metricsResult
	seen := map[uint64]struct{}{}
	for row := 0; row < f.Rows(); row++ {
		b := labels.NewBuilder(labels.EmptyLabels())
		for _, fld := range f.Fields {
			if fld.Type().NonNullableType() != data.FieldTypeString {
				continue
			}
			v, ok := fld.ConcreteAt(row)
			if !ok {
				continue
			}
			name := sanitizeName(fld.Name)
			if name == model.JobLabel || name == model.InstanceLabel {
				res.overridden = true
			}
			b.Set(name, v.(string))
		}
		b.Set(model.JobLabel, job)
		b.Set(model.InstanceLabel, instance)

		for _, fld := range f.Fields {
			val, ok := sampleValue(fld, row)
			if !ok {
				continue
			}
			name := m.prefix + sanitizeName(fld.Name)
			if name == upMetricName {
				res.reservedUp = true
				continue
			}
			b.Set(model.MetricNameLabel, name)
			ls := b.Labels()
			h := ls.Hash()
			if _, dup := seen[h]; dup {
				res.duplicates++
				continue
			}
			seen[h] = struct{}{}
			res.samples = append(res.samples, sample{labels: ls, value: val})
			if m.seriesLimit > 0 && len(res.samples) > m.seriesLimit {
				return metricsResult{}, newPollError(reasonSeriesLimit, fmt.Errorf("query returned more than series_limit (%d) series", m.seriesLimit))
			}
		}
	}
	return res, nil
}

func sampleValue(fld *data.Field, row int) (float64, bool) {
	typ := fld.Type().NonNullableType()
	switch {
	case typ == data.FieldTypeBool:
		v, ok := fld.ConcreteAt(row)
		if !ok {
			return 0, false
		}
		if v.(bool) {
			return 1, true
		}
		return 0, true
	case typ.Numeric():
		if _, ok := fld.ConcreteAt(row); !ok {
			return 0, false
		}
		v, err := fld.FloatAt(row)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

func upSample(job, instance string, v float64) sample {
	return sample{
		labels: labels.FromStrings(model.MetricNameLabel, upMetricName, model.JobLabel, job, model.InstanceLabel, instance),
		value:  v,
	}
}

// sanitizeName makes a valid Prometheus label or metric name.
func sanitizeName(s string) string {
	if s == "" {
		return "_"
	}
	var b strings.Builder
	for i, r := range s {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			if i == 0 {
				b.WriteRune('_')
			}
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// tracker holds the series of the last poll of one query. Each poll
// replaces the whole set, so memory stays bounded by one poll.
type tracker struct {
	series map[uint64]labels.Labels
}

func newTracker() *tracker {
	return &tracker{series: map[uint64]labels.Labels{}}
}

// stale returns the tracked series that are not in current.
func (t *tracker) stale(current []sample) []labels.Labels {
	cur := make(map[uint64]struct{}, len(current))
	for _, s := range current {
		cur[s.labels.Hash()] = struct{}{}
	}
	var out []labels.Labels
	for h, ls := range t.series {
		if _, ok := cur[h]; !ok {
			out = append(out, ls)
		}
	}
	return out
}

func (t *tracker) replace(current []sample) {
	t.series = make(map[uint64]labels.Labels, len(current))
	for _, s := range current {
		t.series[s.labels.Hash()] = s.labels
	}
}

func (t *tracker) all() []labels.Labels {
	out := make([]labels.Labels, 0, len(t.series))
	for _, ls := range t.series {
		out = append(out, ls)
	}
	return out
}

func (t *tracker) allExcept(ls labels.Labels) []labels.Labels {
	skip := ls.Hash()
	out := make([]labels.Labels, 0, len(t.series))
	for h, s := range t.series {
		if h != skip {
			out = append(out, s)
		}
	}
	return out
}

func (t *tracker) reset() { t.series = map[uint64]labels.Labels{} }

func (t *tracker) len() int { return len(t.series) }
