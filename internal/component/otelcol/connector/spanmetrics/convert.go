package spanmetrics

// Alloy writes the aggregation temporality without upstream's
// AGGREGATION_TEMPORALITY_ prefix.
const (
	aggregationTemporalityCumulative = "CUMULATIVE"
	aggregationTemporalityDelta      = "DELTA"
)

const (
	upstreamAggregationTemporalityCumulative = "AGGREGATION_TEMPORALITY_CUMULATIVE"
	upstreamAggregationTemporalityDelta      = "AGGREGATION_TEMPORALITY_DELTA"
)

func aggregationTemporalityToUpstream(v string) string {
	switch v {
	case aggregationTemporalityCumulative:
		return upstreamAggregationTemporalityCumulative
	case aggregationTemporalityDelta:
		return upstreamAggregationTemporalityDelta
	}
	return v
}

func aggregationTemporalityFromUpstream(v string) string {
	switch v {
	case upstreamAggregationTemporalityCumulative:
		return aggregationTemporalityCumulative
	case upstreamAggregationTemporalityDelta:
		return aggregationTemporalityDelta
	}
	return v
}

func timestampCacheSizeToUpstream(v int) *int { return new(v) }

func timestampCacheSizeFromUpstream(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
