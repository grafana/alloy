package wal

import (
	"errors"
	"fmt"
	"io"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/wlog"

	"github.com/grafana/alloy/internal/loki/util"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// readWAL will read all entries in the WAL located under dir.
// Used in tests.
func readWAL(dir string) ([]loki.Entry, error) {
	reader, closeFn, err := newWalReader(dir, -1)
	if err != nil {
		return nil, err
	}
	defer func() { closeFn.Close() }()

	seenSeries := make(map[uint64]model.LabelSet)
	seenEntries := []loki.Entry{}

	for reader.Next() {
		var walRec = Record{}
		bytes := reader.Record()
		err = DecodeRecord(bytes, &walRec)
		if err != nil {
			return nil, fmt.Errorf("error decoding wal record: %w", err)
		}

		// first read series
		for _, series := range walRec.Series {
			if _, ok := seenSeries[uint64(series.Ref)]; !ok {
				seenSeries[uint64(series.Ref)] = util.MapToModelLabelSet(series.Labels.Map())
			}
		}

		for _, entries := range walRec.RefEntries {
			for _, entry := range entries.Entries {
				labels, ok := seenSeries[uint64(entries.Ref)]
				if !ok {
					return nil, fmt.Errorf("found entry without matching series")
				}
				seenEntries = append(seenEntries, loki.Entry{
					Labels: labels,
					Entry:  entry,
				})
			}
		}

		// reset entry
		walRec.Series = walRec.Series[:]
		walRec.RefEntries = walRec.RefEntries[:]
	}

	return seenEntries, nil
}

func newWalReader(dir string, startSegment int) (*wlog.Reader, io.Closer, error) {
	var (
		segmentReader io.ReadCloser
		err           error
	)
	if startSegment < 0 {
		segmentReader, err = wlog.NewSegmentsReader(dir)
		if err != nil {
			return nil, nil, err
		}
	} else {
		first, last, err := wlog.Segments(dir)
		if err != nil {
			return nil, nil, err
		}
		if startSegment > last {
			return nil, nil, errors.New("start segment is beyond the last WAL segment")
		}
		if first > startSegment {
			startSegment = first
		}
		segmentReader, err = wlog.NewSegmentsRangeReader(wlog.SegmentRange{
			Dir:   dir,
			First: startSegment,
			Last:  -1, // Till the end.
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return wlog.NewReader(segmentReader), segmentReader, nil
}
