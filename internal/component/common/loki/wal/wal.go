package wal

import (
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/tsdb/wlog"
	"github.com/prometheus/prometheus/util/compression"
)

// WAL is an interface that allows us to abstract ourselves from Prometheus WAL implementation.
type WAL interface {
	// Log marshals the records and writes it into the WAL.
	Log(*Record) error

	Sync() error
	Dir() string
	Close()
	NextSegment() (int, error)
}

type wrapper struct {
	wal *wlog.WL
}

// New creates a new wrapper, instantiating the actual wlog.WL underneath.
func New(logger *slog.Logger, registerer prometheus.Registerer, dir string) (WAL, error) {
	// TODO: We should fine-tune the WAL instantiated here to allow some buffering of written entries, but not written to disk
	// yet. This will attest for the lack of buffering in the channel Writer exposes.
	tsdbWAL, err := wlog.NewSize(logger, registerer, dir, wlog.DefaultSegmentSize, compression.Snappy)
	if err != nil {
		return nil, fmt.Errorf("failed to create tsdb WAL: %w", err)
	}
	return &wrapper{wal: tsdbWAL}, nil
}

// Close closes the underlying wal, flushing pending writes and closing the active segment. Safe to call more than once
func (w *wrapper) Close() {
	// Avoid checking the error since it's safe to call Close more than once on wlog.WL
	_ = w.wal.Close()
}

func (w *wrapper) Log(record *Record) error {
	if record == nil || (len(record.Series) == 0 && len(record.RefEntries) == 0) {
		return nil
	}

	seriesBuf := getBytes()
	entriesBuf := getBytes()
	defer func() {
		putBytes(seriesBuf)
		putBytes(entriesBuf)
	}()

	*seriesBuf = record.EncodeSeries(*seriesBuf)
	*entriesBuf = record.EncodeEntries(CurrentEntriesRec, *entriesBuf)
	// Always write series then entries
	return w.wal.Log(*seriesBuf, *entriesBuf)
}

// Sync flushes changes to disk. Mainly to be used for testing.
func (w *wrapper) Sync() error {
	return w.wal.Sync()
}

// Dir returns the path to the WAL directory.
func (w *wrapper) Dir() string {
	return w.wal.Dir()
}

// NextSegment closes the current segment synchronously. Mainly used for testing.
func (w *wrapper) NextSegment() (int, error) {
	return w.wal.NextSegmentSync()
}
