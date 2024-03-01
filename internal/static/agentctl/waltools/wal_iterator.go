package waltools

import (
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
)

// walIterate iterates over the latest checkpoint in the provided WAL and all
// of the segments in the WAL and calls f for each of them.
func walIterate(w *wlog.WL, f func(r *wlog.Reader) error) error {
	checkpoint, checkpointIdx, err := wlog.LastCheckpoint(w.Dir())
	if err != nil && err != record.ErrNotFound {
		return err
	}

	startIdx, last, err := wlog.Segments(w.Dir())
	if err != nil {
		return err
	}

	if checkpoint != "" {
		sr, err := wlog.NewSegmentsReader(checkpoint)
		if err != nil {
			return err
		}
		err = f(wlog.NewReader(sr))
		_ = sr.Close()
		if err != nil {
			return err
		}

		startIdx = checkpointIdx + 1
	}

	for i := startIdx; i <= last; i++ {
		s, err := wlog.OpenReadSegment(wlog.SegmentName(w.Dir(), i))
		if err != nil {
			return err
		}
		sr := wlog.NewSegmentBufReader(s)
		err = f(wlog.NewReader(sr))
		_ = sr.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
