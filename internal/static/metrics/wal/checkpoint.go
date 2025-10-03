package wal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/fileutil"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Checkpoint is a modified wlog.Checkpoint implementation targeted toward the needs of Storage and remote.QueueManager
// Storage needs to know series that exist in the WAL and the last timestamp they were sent - which is stripeSeries and deleted
// remote.QueueManager needs to know the series that might exist in the WAL - which is stripeSeries and deleted (TODO this might need to include metadata too)
func Checkpoint(logger log.Logger, w *wlog.WL, atIndex int, series *stripeSeries, deleted map[chunks.HeadSeriesRef]int) error {
	level.Info(logger).Log("msg", "Creating checkpoint from WAL")

	dir, idx, err := wlog.LastCheckpoint(w.Dir())
	if err != nil && !errors.Is(err, record.ErrNotFound) {
		return fmt.Errorf("find last checkpoint: %w", err)
	}

	if idx >= atIndex {
		level.Info(logger).Log("msg", "checkpoint already exists", "dir", dir, "index", idx, "requested_index", atIndex)
		return nil
	}

	// TODO cleanup old temp checkpoints
	cpdir := checkpointDir(w.Dir(), atIndex)
	cpdirtmp := cpdir + ".tmp"
	if err := os.RemoveAll(cpdirtmp); err != nil {
		return fmt.Errorf("remove previous temporary checkpoint dir: %w", err)
	}

	if err := os.MkdirAll(cpdirtmp, 0o777); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	cp, err := wlog.New(nil, nil, cpdirtmp, w.CompressionType())
	if err != nil {
		return fmt.Errorf("open checkpoint: %w", err)
	}
	defer func() {
		os.RemoveAll(cpdirtmp)
		cp.Close()
	}()

	var (
		enc          record.Encoder
		seriesBytes  []byte
		samplesBytes []byte
	)
	batchSize := 1000
	seriesRecords := make([]record.RefSeries, batchSize)
	sampleRecords := make([]record.RefSample, batchSize)
	n := 0

	flushRecords := func(withSamples bool) error {
		seriesBytes = enc.Series(seriesRecords, seriesBytes)
		if withSamples {
			samplesBytes = enc.Samples(sampleRecords, samplesBytes)
			if err := cp.Log(seriesBytes, samplesBytes); err != nil {
				return fmt.Errorf("flush records: %w", err)
			}
		} else {
			if err := cp.Log(seriesBytes); err != nil {
				return fmt.Errorf("flush records: %w", err)
			}
		}

		seriesBytes, samplesBytes = seriesBytes[:0], samplesBytes[:0]
		n = 0
		return nil
	}
	for ms := range series.Iterate() {
		// If we filled the buffers, write them out and reset.
		if n == batchSize {
			if err := flushRecords(true); err != nil {
				return fmt.Errorf("flushing active series: %w", err)
			}
		}

		seriesRecords[n] = record.RefSeries{Ref: ms.ref, Labels: ms.lset}
		// Sample value is irrelevant, we only need the timestamp
		sampleRecords[n] = record.RefSample{Ref: ms.ref, T: ms.lastTs, V: 0}
		n++
	}
	// Clear the last batch if we have one
	if n != 0 {
		if err := flushRecords(true); err != nil {
			return fmt.Errorf("flush records: %w", err)
		}
	}

	// Now write out the deleted records. We don't care about timestamps here, so no samples.
	for ref := range deleted {
		// If we filled the buffers, write them out and reset.
		if n == batchSize {
			if err := flushRecords(false); err != nil {
				return fmt.Errorf("flushing deleted series: %w", err)
			}
		}
		seriesRecords[n] = record.RefSeries{Ref: ref, Labels: nil}
		n++
	}
	// Clear the last batch if we have one
	if n != 0 {
		if err := flushRecords(false); err != nil {
			return fmt.Errorf("flush records: %w", err)
		}
	}

	if err := cp.Close(); err != nil {
		return fmt.Errorf("close checkpoint: %w", err)
	}

	// Sync temporary directory before rename.
	df, err := fileutil.OpenDir(cpdirtmp)
	if err != nil {
		return fmt.Errorf("open temporary checkpoint directory: %w", err)
	}
	if err := df.Sync(); err != nil {
		df.Close()
		return fmt.Errorf("sync temporary checkpoint directory: %w", err)
	}
	if err = df.Close(); err != nil {
		return fmt.Errorf("close temporary checkpoint directory: %w", err)
	}

	if err := fileutil.Replace(cpdirtmp, cpdir); err != nil {
		return fmt.Errorf("rename checkpoint directory: %w", err)
	}

	return nil
}

func checkpointDir(dir string, i int) string {
	return filepath.Join(dir, fmt.Sprintf(wlog.CheckpointPrefix+"%08d", i))
}
