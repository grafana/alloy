package network

import (
	"net/http"

	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
)

// recordStats determines what values to send to the stats function. This allows for any
// number of metrics/signals libraries to be used. Prometheus, OTel, and any other.
func recordStats(series []*types.TimeSeriesBinary, isMeta bool, stats func(s types.NetworkStats), r sendResult, bytesSent int) {
	switch {
	case r.networkError:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				NetworkSamplesFailed: getSeriesCount(series),
			},
			Histogram: types.CategoryStats{
				NetworkSamplesFailed: getHistogramCount(series),
			},
		})
	case r.successful:
		// Need to grab the newest series.
		var newestTS int64
		for _, ts := range series {
			if ts.TS > newestTS {
				newestTS = ts.TS
			}
		}
		var sampleBytesSent int
		var metaBytesSent int
		// Each loop is explicitly a normal signal or metadata sender.
		if isMeta {
			metaBytesSent = bytesSent
		} else {
			sampleBytesSent = bytesSent
		}
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				SeriesSent: getSeriesCount(series),
			},
			Histogram: types.CategoryStats{
				SeriesSent: getHistogramCount(series),
			},
			MetadataBytes:   metaBytesSent,
			SeriesBytes:     sampleBytesSent,
			NewestTimestamp: newestTS,
		})
	case r.statusCode == http.StatusTooManyRequests:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				RetriedSamples:    getSeriesCount(series),
				RetriedSamples429: getSeriesCount(series),
			},
			Histogram: types.CategoryStats{
				RetriedSamples:    getHistogramCount(series),
				RetriedSamples429: getHistogramCount(series),
			},
		})
	case r.statusCode/100 == 5:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				RetriedSamples5XX: getSeriesCount(series),
			},
			Histogram: types.CategoryStats{
				RetriedSamples5XX: getHistogramCount(series),
			},
		})
	case r.statusCode != 200:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				FailedSamples: getSeriesCount(series),
			},
			Histogram: types.CategoryStats{
				FailedSamples: getHistogramCount(series),
			},
			Metadata: types.CategoryStats{
				FailedSamples: getMetadataCount(series),
			},
		})
	}

}

func getSeriesCount(tss []*types.TimeSeriesBinary) int {
	cnt := 0
	for _, ts := range tss {
		// This is metadata
		if isMetadata(ts) {
			continue
		}
		if ts.Histograms.Histogram == nil && ts.Histograms.FloatHistogram == nil {
			cnt++
		}
	}
	return cnt
}

func getHistogramCount(tss []*types.TimeSeriesBinary) int {
	cnt := 0
	for _, ts := range tss {
		if isMetadata(ts) {
			continue
		}
		if ts.Histograms.Histogram != nil || ts.Histograms.FloatHistogram != nil {
			cnt++
		}
	}
	return cnt
}
