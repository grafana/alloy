package dbo11yreceiver

import (
	"math"
	"time"

	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const scopeName = "github.com/grafana/alloy/receiver/dbo11yreceiver"

// POC-HACK: Gather() output is converted by hand instead of through
// prometheus/opentelemetry-collector-bridge, whose converter is unexported
// and whose receiver owns one registry and its own service.name (F-17).

// appendMetrics converts one database's gathered families into a new
// ResourceMetrics of dest. targetLabels are the database's labels after
// MetricRules and the per-database rules: they set the resource and are added
// to every datapoint the way a Prometheus scrape adds target labels.
// fallbackStart is the start timestamp of cumulative points whose metric has
// no created timestamp.
func appendMetrics(dest pmetric.Metrics, families []*dto.MetricFamily, targetLabels map[string]string, now, fallbackStart time.Time) {
	rm := dest.ResourceMetrics().AppendEmpty()
	putResource(rm.Resource().Attributes(), targetLabels)
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(scopeName)

	for _, fam := range families {
		if len(fam.GetMetric()) == 0 {
			continue
		}
		m := sm.Metrics().AppendEmpty()
		m.SetName(fam.GetName())
		m.SetDescription(fam.GetHelp())
		m.SetUnit(fam.GetUnit())

		switch fam.GetType() {
		case dto.MetricType_COUNTER:
			sum := m.SetEmptySum()
			sum.SetIsMonotonic(true)
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			for _, pm := range fam.GetMetric() {
				dp := sum.DataPoints().AppendEmpty()
				dp.SetDoubleValue(pm.GetCounter().GetValue())
				setStart(dp.SetStartTimestamp, pm.GetCounter().GetCreatedTimestamp(), fallbackStart)
				setPoint(dp.Attributes(), dp.SetTimestamp, pm, targetLabels, now)
			}
		case dto.MetricType_HISTOGRAM, dto.MetricType_GAUGE_HISTOGRAM:
			h := m.SetEmptyHistogram()
			h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			for _, pm := range fam.GetMetric() {
				ph := pm.GetHistogram()
				dp := h.DataPoints().AppendEmpty()
				dp.SetCount(ph.GetSampleCount())
				dp.SetSum(ph.GetSampleSum())
				var prev uint64
				for _, b := range ph.GetBucket() {
					if math.IsInf(b.GetUpperBound(), +1) {
						continue
					}
					dp.ExplicitBounds().Append(b.GetUpperBound())
					dp.BucketCounts().Append(b.GetCumulativeCount() - prev)
					prev = b.GetCumulativeCount()
				}
				// The +Inf bucket.
				dp.BucketCounts().Append(ph.GetSampleCount() - prev)
				setStart(dp.SetStartTimestamp, ph.GetCreatedTimestamp(), fallbackStart)
				setPoint(dp.Attributes(), dp.SetTimestamp, pm, targetLabels, now)
			}
		case dto.MetricType_SUMMARY:
			s := m.SetEmptySummary()
			for _, pm := range fam.GetMetric() {
				ps := pm.GetSummary()
				dp := s.DataPoints().AppendEmpty()
				dp.SetCount(ps.GetSampleCount())
				dp.SetSum(ps.GetSampleSum())
				for _, q := range ps.GetQuantile() {
					qv := dp.QuantileValues().AppendEmpty()
					qv.SetQuantile(q.GetQuantile())
					qv.SetValue(q.GetValue())
				}
				setStart(dp.SetStartTimestamp, ps.GetCreatedTimestamp(), fallbackStart)
				setPoint(dp.Attributes(), dp.SetTimestamp, pm, targetLabels, now)
			}
		default: // GAUGE and UNTYPED
			g := m.SetEmptyGauge()
			for _, pm := range fam.GetMetric() {
				dp := g.DataPoints().AppendEmpty()
				if fam.GetType() == dto.MetricType_GAUGE {
					dp.SetDoubleValue(pm.GetGauge().GetValue())
				} else {
					dp.SetDoubleValue(pm.GetUntyped().GetValue())
				}
				setPoint(dp.Attributes(), dp.SetTimestamp, pm, targetLabels, now)
			}
		}
	}
}

// setStart uses the metric's created timestamp when client_golang has one.
// POC-HACK: otherwise it falls back to when the database connected, which is
// only right for metrics created by that Connect.
func setStart(set func(pcommon.Timestamp), created *timestamppb.Timestamp, fallback time.Time) {
	if created != nil && created.IsValid() {
		set(pcommon.NewTimestampFromTime(created.AsTime()))
		return
	}
	set(pcommon.NewTimestampFromTime(fallback))
}

// setPoint sets the timestamp and attributes of one datapoint: the metric's
// labels, then the target labels. As in a Prometheus scrape with
// honor_labels false, a metric label that clashes with a target label is kept
// as exported_<name>.
func setPoint(attrs pcommon.Map, setTS func(pcommon.Timestamp), pm *dto.Metric, targetLabels map[string]string, now time.Time) {
	ts := now
	if pm.TimestampMs != nil {
		ts = time.UnixMilli(pm.GetTimestampMs())
	}
	setTS(pcommon.NewTimestampFromTime(ts))

	for _, lp := range pm.GetLabel() {
		name := lp.GetName()
		if _, clash := targetLabels[name]; clash {
			name = "exported_" + name
		}
		attrs.PutStr(name, lp.GetValue())
	}
	for name, value := range targetLabels {
		if datapointLabel(name) {
			attrs.PutStr(name, value)
		}
	}
}
