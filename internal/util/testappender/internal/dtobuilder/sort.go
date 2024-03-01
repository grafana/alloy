package dtobuilder

import (
	"sort"

	dto "github.com/prometheus/client_model/go"
)

// sortMetricFamilies sorts metric families, the metrics inside them, and the
// summaries and histograms inside the individual metrics.
func sortMetricFamilies(mf []*dto.MetricFamily) {
	sort.Slice(mf, func(i, j int) bool {
		return mf[i].GetName() < mf[j].GetName()
	})

	for _, family := range mf {
		sortMetrics(family.Metric)
	}
}

// sortMetrics sorts a slice of metrics, followed by the quantiles and
// histogram buckets (if present) inside each metric.
func sortMetrics(mm []*dto.Metric) {
	sort.Slice(mm, func(i, j int) bool {
		return labelsLess(mm[i].GetLabel(), mm[j].GetLabel())
	})

	for _, m := range mm {
		if m.Summary != nil {
			sortQuantiles(m.Summary.GetQuantile())
		}

		if m.Histogram != nil {
			sortBuckets(m.Histogram.GetBucket())
		}
	}
}

// labelsLess implements the sort.Slice "less" function, returning true if a
// should appear before b in a list of sorted labels.
func labelsLess(a, b []*dto.LabelPair) bool {
	for i := 0; i < len(a); i++ {
		// If all labels have matched but we've gone past the length
		// of b, that means that a > b.
		// If we've gone past the length of b, then a has more labels
		if i >= len(b) {
			return false
		}

		switch {
		case a[i].GetName() != b[i].GetName():
			return a[i].GetName() < b[i].GetName()
		case a[i].GetValue() != b[i].GetValue():
			return a[i].GetValue() < b[i].GetValue()
		}
	}

	// Either they're fully equal or a < b, so we return true either way.
	return true
}

// sortQuantiles sorts a slice of quantiles.
func sortQuantiles(qq []*dto.Quantile) {
	sort.Slice(qq, func(i, j int) bool {
		return qq[i].GetQuantile() < qq[j].GetQuantile()
	})
}

// sortBuckets sorts a slice of buckets.
func sortBuckets(bb []*dto.Bucket) {
	sort.Slice(bb, func(i, j int) bool {
		return bb[i].GetUpperBound() < bb[j].GetUpperBound()
	})
}
