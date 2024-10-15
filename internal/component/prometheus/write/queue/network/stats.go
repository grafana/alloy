package network

import (
	"net/http"

	"github.com/grafana/alloy/internal/component/prometheus/write/queue/types"
)

// recordStats determines what values to send to the stats function. This allows for any
// number of metrics/signals libraries to be used. Prometheus, OTel, and any other.
func recordStats(series []*types.TimeSeriesBinary, isMeta bool, stats func(s types.NetworkStats), r sendResult, bytesSent int) {
	seriesCount := getSeriesCount(series)
	histogramCount := getHistogramCount(series)
	metadataCount := getMetadataCount(series)
	switch {
	case r.networkError:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				NetworkSamplesFailed: seriesCount,
			},
			Histogram: types.CategoryStats{
				NetworkSamplesFailed: histogramCount,
			},
			Metadata: types.CategoryStats{
				NetworkSamplesFailed: metadataCount,
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
				SeriesSent: seriesCount,
			},
			Histogram: types.CategoryStats{
				SeriesSent: histogramCount,
			},
			Metadata: types.CategoryStats{
				SeriesSent: metadataCount,
			},
			MetadataBytes:   metaBytesSent,
			SeriesBytes:     sampleBytesSent,
			NewestTimestamp: newestTS,
		})
	case r.statusCode == http.StatusTooManyRequests:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				RetriedSamples:    seriesCount,
				RetriedSamples429: seriesCount,
			},
			Histogram: types.CategoryStats{
				RetriedSamples:    histogramCount,
				RetriedSamples429: histogramCount,
			},
			Metadata: types.CategoryStats{
				RetriedSamples:    metadataCount,
				RetriedSamples429: metadataCount,
			},
		})
	case r.statusCode/100 == 5:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				RetriedSamples5XX: seriesCount,
			},
			Histogram: types.CategoryStats{
				RetriedSamples5XX: histogramCount,
			},
			Metadata: types.CategoryStats{
				RetriedSamples: metadataCount,
			},
		})
	case r.statusCode != 200:
		stats(types.NetworkStats{
			Series: types.CategoryStats{
				FailedSamples: seriesCount,
			},
			Histogram: types.CategoryStats{
				FailedSamples: histogramCount,
			},
			Metadata: types.CategoryStats{
				FailedSamples: metadataCount,
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
