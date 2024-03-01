package waltools

import (
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
)

// SampleStats are statistics for samples for a series within the WAL. Each
// instance represents a unique series based on its labels, and holds the range
// of timestamps found for all samples including the total number of samples
// for that series.
type SampleStats struct {
	Labels  labels.Labels
	From    time.Time
	To      time.Time
	Samples int64
}

// FindSamples searches the WAL and returns a summary of samples of series
// matching the given label selector.
func FindSamples(walDir string, selectorStr string) ([]*SampleStats, error) {
	w, err := wlog.Open(nil, walDir)
	if err != nil {
		return nil, err
	}
	defer w.Close()

	selector, err := parser.ParseMetricSelector(selectorStr)
	if err != nil {
		return nil, err
	}

	var (
		labelsByRef = make(map[chunks.HeadSeriesRef]labels.Labels)

		minTSByRef       = make(map[chunks.HeadSeriesRef]int64)
		maxTSByRef       = make(map[chunks.HeadSeriesRef]int64)
		sampleCountByRef = make(map[chunks.HeadSeriesRef]int64)
	)

	// get the references matching label selector
	err = walIterate(w, func(r *wlog.Reader) error {
		return collectSeries(r, selector, labelsByRef)
	})
	if err != nil {
		return nil, fmt.Errorf("could not collect series: %w", err)
	}

	// find related samples
	err = walIterate(w, func(r *wlog.Reader) error {
		return collectSamples(r, labelsByRef, minTSByRef, maxTSByRef, sampleCountByRef)
	})
	if err != nil {
		return nil, fmt.Errorf("could not collect samples: %w", err)
	}

	series := make([]*SampleStats, 0, len(labelsByRef))
	for ref, labels := range labelsByRef {
		series = append(series, &SampleStats{
			Labels:  labels,
			Samples: sampleCountByRef[ref],
			From:    timestamp.Time(minTSByRef[ref]),
			To:      timestamp.Time(maxTSByRef[ref]),
		})
	}

	return series, nil
}

func collectSeries(r *wlog.Reader, selector labels.Selector, labelsByRef map[chunks.HeadSeriesRef]labels.Labels) error {
	var dec record.Decoder

	for r.Next() {
		rec := r.Record()

		switch dec.Type(rec) {
		case record.Series:
			series, err := dec.Series(rec, nil)
			if err != nil {
				return err
			}
			for _, s := range series {
				if selector.Matches(s.Labels) {
					labelsByRef[s.Ref] = s.Labels.Copy()
				}
			}
		}
	}

	return r.Err()
}

func collectSamples(r *wlog.Reader, labelsByRef map[chunks.HeadSeriesRef]labels.Labels, minTS, maxTS, sampleCount map[chunks.HeadSeriesRef]int64) error {
	var dec record.Decoder

	for r.Next() {
		rec := r.Record()

		switch dec.Type(rec) {
		case record.Samples:
			samples, err := dec.Samples(rec, nil)
			if err != nil {
				return err
			}

			for _, s := range samples {
				// skip unmatched series
				if _, ok := labelsByRef[s.Ref]; !ok {
					continue
				}

				// determine min/max TS
				if ts, ok := minTS[s.Ref]; !ok || ts > s.T {
					minTS[s.Ref] = s.T
				}
				if ts, ok := maxTS[s.Ref]; !ok || ts < s.T {
					maxTS[s.Ref] = s.T
				}

				sampleCount[s.Ref]++
			}
		}
	}

	return r.Err()
}
