package waltools

import (
	"math"
	"time"

	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
)

// WALStats stores statistics on the whole WAL.
type WALStats struct {
	// From holds the first timestamp for the oldest sample found within the WAL.
	From time.Time

	// To holds the last timestamp for the newest sample found within the WAL.
	To time.Time

	// CheckpointNumber is the segment number of the most recently created
	// checkpoint.
	CheckpointNumber int

	// FirstSegment is the segment number of the first (oldest) non-checkpoint
	// segment file found within the WAL folder.
	FirstSegment int

	// FirstSegment is the segment number of the last (newest) non-checkpoint
	// segment file found within the WAL folder.
	LastSegment int

	// InvalidRefs is the number of samples with a ref ID to which there is no
	// series defined.
	InvalidRefs int

	// HashCollisions is the total number of times there has been a hash
	// collision. A hash collision is any instance in which a hash of labels
	// is defined by two ref IDs.
	//
	// For the Grafana Agent, a hash collision has no negative side effects
	// on data sent to the remote_write endpoint but may have a noticeable impact
	// on memory while the collision exists.
	HashCollisions int

	// Targets holds stats on specific scrape targets.
	Targets []WALTargetStats
}

// Series returns the number of series across all targets.
func (s WALStats) Series() int {
	var series int
	for _, t := range s.Targets {
		series += t.Series
	}
	return series
}

// Samples returns the number of Samples across all targets.
func (s WALStats) Samples() int {
	var samples int
	for _, t := range s.Targets {
		samples += t.Samples
	}
	return samples
}

// WALTargetStats aggregates statistics on scrape targets across the entirety
// of the WAL and its checkpoints.
type WALTargetStats struct {
	// Job corresponds to the "job" label on the scraped target.
	Job string

	// Instance corresponds to the "instance" label on the scraped target.
	Instance string

	// Series is the total number of series for the scraped target. It is
	// equivalent to the total cardinality.
	Series int

	// Samples is the total number of samples for the scraped target.
	Samples int
}

// CalculateStats calculates the statistics of the WAL for the given directory.
// walDir must be a folder containing segment files and checkpoint directories.
func CalculateStats(walDir string) (WALStats, error) {
	w, err := wlog.Open(nil, walDir)
	if err != nil {
		return WALStats{}, err
	}
	defer w.Close()

	return newWALStatsCalculator(w).Calculate()
}

type walStatsCalculator struct {
	w *wlog.WL

	fromTime    int64
	toTime      int64
	invalidRefs int

	stats []*WALTargetStats

	statsLookup map[chunks.HeadSeriesRef]*WALTargetStats

	// hash -> # ref IDs with that hash
	hashInstances map[uint64]int
}

func newWALStatsCalculator(w *wlog.WL) *walStatsCalculator {
	return &walStatsCalculator{
		w:             w,
		fromTime:      math.MaxInt64,
		statsLookup:   make(map[chunks.HeadSeriesRef]*WALTargetStats),
		hashInstances: make(map[uint64]int),
	}
}

func (c *walStatsCalculator) Calculate() (WALStats, error) {
	var (
		stats WALStats
		err   error
	)

	_, checkpointIdx, err := wlog.LastCheckpoint(c.w.Dir())
	if err != nil && err != record.ErrNotFound {
		return stats, err
	}

	firstSegment, lastSegment, err := wlog.Segments(c.w.Dir())
	if err != nil {
		return stats, err
	}

	stats.FirstSegment = firstSegment
	stats.LastSegment = lastSegment
	stats.CheckpointNumber = checkpointIdx

	// Iterate over the WAL and collect stats. This must be done before the rest
	// of the function as readWAL populates internal state used for calculating
	// stats.
	err = walIterate(c.w, c.readWAL)
	if err != nil {
		return stats, err
	}

	// Fill in the rest of the stats
	stats.From = timestamp.Time(c.fromTime)
	stats.To = timestamp.Time(c.toTime)
	stats.InvalidRefs = c.invalidRefs

	for _, hashCount := range c.hashInstances {
		if hashCount > 1 {
			stats.HashCollisions++
		}
	}

	for _, tgt := range c.stats {
		stats.Targets = append(stats.Targets, *tgt)
	}

	return stats, nil
}

func (c *walStatsCalculator) readWAL(r *wlog.Reader) error {
	var dec record.Decoder

	for r.Next() {
		rec := r.Record()

		// We ignore other record types here; we only write records and samples
		// but we don't want to return an error for an unexpected record type;
		// doing so would prevent users from getting stats on a traditional
		// Prometheus WAL, which would be nice to support.
		switch dec.Type(rec) {
		case record.Series:
			series, err := dec.Series(rec, nil)
			if err != nil {
				return err
			}
			for _, s := range series {
				var (
					jobLabel      = s.Labels.Get("job")
					instanceLabel = s.Labels.Get("instance")
				)

				// Find or create the WALTargetStats for this job/instance pair.
				var stats *WALTargetStats
				for _, wts := range c.stats {
					if wts.Job == jobLabel && wts.Instance == instanceLabel {
						stats = wts
						break
					}
				}
				if stats == nil {
					stats = &WALTargetStats{Job: jobLabel, Instance: instanceLabel}
					c.stats = append(c.stats, stats)
				}

				// Every time we get a new series, we want to increment the series
				// count for the specific job/instance pair, store the ref ID so
				// samples can modify the stats, and then store the hash of our
				// labels to detect collisions (or flapping series).
				stats.Series++
				c.statsLookup[s.Ref] = stats
				c.hashInstances[s.Labels.Hash()]++
			}
		case record.Samples:
			samples, err := dec.Samples(rec, nil)
			if err != nil {
				return err
			}
			for _, s := range samples {
				if s.T < c.fromTime {
					c.fromTime = s.T
				}
				if s.T > c.toTime {
					c.toTime = s.T
				}

				stats := c.statsLookup[s.Ref]
				if stats == nil {
					c.invalidRefs++
					continue
				}
				stats.Samples++
			}
		}
	}

	return r.Err()
}

// BySeriesCount can sort a slice of target stats by the count of
// series. The slice is sorted in descending order.
type BySeriesCount []WALTargetStats

func (s BySeriesCount) Len() int           { return len(s) }
func (s BySeriesCount) Less(i, j int) bool { return s[i].Series > s[j].Series }
func (s BySeriesCount) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
