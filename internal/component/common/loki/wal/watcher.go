package wal

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"

	"github.com/grafana/alloy/internal/component/common/loki/wal/internal"
)

const (
	segmentCheckPeriod = 100 * time.Millisecond
)

// Based in the implementation of prometheus WAL watcher
// https://github.com/prometheus/prometheus/blob/main/tsdb/wlog/watcher.go. Includes some changes to make it suitable
// for log WAL entries, but also, the writeTo surface has been implemented according to the actions necessary for
// Promtail's WAL.

// Reader is a dependency interface to inject generic WAL readers into the Watcher.
type Reader interface {
	Next() bool
	Err() error
	Record() []byte
}

// WriteCleanup is responsible for cleaning up resources used in the process of reading the WAL.
type WriteCleanup interface {
	// SeriesReset is called to notify that segments have been deleted. The argument of the call
	// means that all segments with a number lower or equal than segmentNum are safe to be reclaimed.
	SeriesReset(segmentNum int)
}

// WriteTo is an interface used by the Watcher to send the samples it's read from the WAL on to
// somewhere else, or clean them up. It's the intermediary between all information read by the Watcher
// and the final destination.
//
// Based on https://github.com/prometheus/prometheus/blob/main/tsdb/wlog/watcher.go#L46
type WriteTo interface {
	// WriteCleanup is used to allow the Watcher to react upon being notified of WAL cleanup events, such as segments
	// being reclaimed.
	WriteCleanup

	// StoreSeries is called when series are found in WAL entries by the watcher, alongside with the segmentNum they were
	// found in.
	StoreSeries(series []record.RefSeries, segmentNum int)

	AppendEntries(entries RefEntries, segmentNum int) error
}

// Marker allows the Watcher to start from a specific segment in the WAL.
// Implementers can use this interface to save and restore save points.
type Marker interface {
	// LastMarkedSegment should return the last segment stored in the marker.
	// Must return -1 if there is no mark.
	//
	// The Watcher will start reading the first segment whose value is greater
	// than the return value.
	LastMarkedSegment() int
}

type Watcher struct {
	// id identifies the Watcher. Used when one Watcher is instantiated per remote write client, to be able to track to whom
	// the metric/log line corresponds.
	id string

	actions    WriteTo
	readNotify chan struct{}
	done       chan struct{}
	state      *internal.WatcherState
	walDir     string
	logger     *slog.Logger
	MaxSegment int

	metrics      *WatcherMetrics
	minReadFreq  time.Duration
	maxReadFreq  time.Duration
	drainTimeout time.Duration
	marker       Marker
	savedSegment int
}

// NewWatcher creates a new Watcher.
func NewWatcher(walDir, id string, metrics *WatcherMetrics, writeTo WriteTo, logger *slog.Logger, config WatchConfig, marker Marker) *Watcher {
	return &Watcher{
		walDir:       walDir,
		id:           id,
		actions:      writeTo,
		readNotify:   make(chan struct{}),
		state:        internal.NewWatcherState(logger),
		done:         make(chan struct{}),
		MaxSegment:   -1,
		marker:       marker,
		savedSegment: -1,
		logger:       logger,
		metrics:      metrics,
		minReadFreq:  config.MinReadFrequency,
		maxReadFreq:  config.MaxReadFrequency,
		drainTimeout: config.DrainTimeout,
	}
}

// Start runs the watcher main loop.
func (w *Watcher) Start() {
	w.metrics.watchersRunning.WithLabelValues().Inc()
	go w.mainLoop()
}

// mainLoop retries when there's an error reading a specific segment or advancing one, but leaving a bigger time in-between
// retries.
func (w *Watcher) mainLoop() {
	defer close(w.done)
	for !w.state.IsStopping() {
		if w.marker != nil {
			w.savedSegment = w.marker.LastMarkedSegment()
			w.logger.Debug("last saved segment", "segment", w.savedSegment)
		}

		err := w.run()
		if err != nil {
			w.logger.Error("error tailing WAL", "err", err)
		}

		if w.state.IsDraining() && errors.Is(err, os.ErrNotExist) {
			w.logger.Info("reached non existing segment while draining, assuming end of WAL")
			// since we've reached the end of the WAL, and the Watcher is draining, promptly transition to stopping state
			// so the watcher can stoppingSignal early
			w.state.Transition(internal.StateStopping)
		}

		select {
		case <-w.state.WaitForStopping():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// Run the watcher, which will tail the WAL until the quit channel is closed or an error case is hit.
func (w *Watcher) run() error {
	_, lastSegment, err := w.firstAndLast()
	if err != nil {
		return fmt.Errorf("wal.Segments: %w", err)
	}

	currentSegment := lastSegment

	// if the marker contains a valid segment number stored, and we correctly find the segment that follows that one,
	// start tailing from there.
	if nextToMarkedSegment, err := w.findNextSegmentFor(w.savedSegment); w.savedSegment != -1 && err == nil {
		currentSegment = nextToMarkedSegment
		// keep a separate metric that will help us track when the segment in the marker is used. This should be considered
		// a replay event
		w.metrics.replaySegment.WithLabelValues(w.id).Set(float64(currentSegment))
	} else {
		w.logger.Debug("failed to find segment", "segment", w.savedSegment, "err", err)
	}

	w.logger.Debug("tailing WAL", "currentSegment", currentSegment, "lastSegment", lastSegment)
	for !w.state.IsStopping() {
		w.metrics.currentSegment.WithLabelValues(w.id).Set(float64(currentSegment))

		// On start, we have a pointer to what is the latest segment. On subsequent calls to this function,
		// currentSegment will have been incremented, and we should open that segment.
		if err := w.watch(currentSegment, currentSegment >= lastSegment); err != nil {
			return err
		}

		// For testing: stop when you hit a specific segment.
		if currentSegment == w.MaxSegment {
			return nil
		}

		currentSegment++
	}

	return nil
}

// watch will start reading from the segment identified by segmentNum.
// If an EOF is reached and tail is true, it will keep reading for more WAL records with a wlog.LiveReader. Periodically,
// it will check if there's a new segment, and if positive read the remaining from the current one and return.
// If tail is false, we know the segment we are "watching" over is closed (no further write will occur to it). Then, the
// segment is read fully, any errors are logged as Warnings, and no error is returned.
func (w *Watcher) watch(segmentNum int, tail bool) error {
	w.logger.Debug("watching WAL segment", "currentSegment", segmentNum, "tail", tail)

	segment, err := wlog.OpenReadSegment(wlog.SegmentName(w.walDir, segmentNum))
	if err != nil {
		return err
	}
	defer segment.Close()

	reader := wlog.NewLiveReader(w.logger, nil, segment)

	readTimer := newBackoffTimer(w.minReadFreq, w.maxReadFreq)

	segmentTicker := time.NewTicker(segmentCheckPeriod)
	defer segmentTicker.Stop()

	// If we're replaying the segment we need to know the size of the file to know when to return from watch and move on
	// to the next segment.
	size := int64(math.MaxInt64)
	if !tail {
		// stop segment ticker since we know we'll read the segment fully, and then exit to the next segment loop
		segmentTicker.Stop()
		var err error
		size, err = getSegmentSize(w.walDir, segmentNum)
		if err != nil {
			return fmt.Errorf("error getting segment size: %w", err)
		}
	}

	for {
		select {
		case <-w.state.WaitForStopping():
			return nil

		case <-segmentTicker.C:
			_, last, err := w.firstAndLast()
			if err != nil {
				return fmt.Errorf("segments: %w", err)
			}

			// Check if new segments exists, or we are draining the WAL, which means that either:
			// - This is the last segment, and we can consume it fully because we are draining the WAL
			// - There's a segment after the current one, and we can consume this segment fully as well
			if last <= segmentNum && !w.state.IsDraining() {
				continue
			}

			if w.state.IsDraining() {
				w.logger.Debug("draining segment completely", "segment", segmentNum, "lastSegment", last)
			}

			// We know that there's either a new segment (last > segmentNum), or we are draining the WAL. Either case, read
			// the remaining data from the segmentNum and return from `watch` to read the next one.
			_, err = w.readSegment(reader, segmentNum)

			// io.EOF error are non-fatal since we are consuming the segment till the end
			if !errors.Is(err, io.EOF) {
				return err
			}

			// return after reading the whole segment
			return nil

		// the cases below will unlock the select block, and execute the block below
		// https://github.com/golang/go/issues/23196#issuecomment-353169837
		case <-readTimer.C:
			w.metrics.segmentRead.WithLabelValues(w.id, "timer").Inc()
		case <-w.readNotify:
			w.metrics.segmentRead.WithLabelValues(w.id, "notification").Inc()
		}

		// read from open segment routine
		ok, err := w.readSegment(reader, segmentNum)
		// Ignore all errors reading to end of segment whilst replaying the WAL. This is because when replaying not the
		// last segment, we assume that segment is not written anymore (closed), and the call to readSegment will read
		// to the end of it. If error, log a warning accordingly. After, error or no error, nil is returned so that the
		// caller can continue to the following segment.
		if !tail {
			if err != nil && !errors.Is(err, io.EOF) {
				w.logger.Warn("ignoring error reading to end of segment, may have dropped data", "segment", segmentNum, "err", err)
			} else if reader.Offset() != size {
				w.logger.Warn("expected to have read whole segment, may have dropped data", "segment", segmentNum, "read", reader.Offset(), "size", size)
			}
			return nil
		}

		// io.EOF error are non-fatal since we are tailing the wal
		if !errors.Is(err, io.EOF) {
			return err
		}

		if ok {
			// read ok, reset readTimer to minimum interval
			readTimer.reset()
			continue
		}

		readTimer.backoff()
	}
}

// Read entries from a segment, decode them and dispatch them.
func (w *Watcher) readSegment(r *wlog.LiveReader, segmentNum int) (bool, error) {
	var readData bool

	for r.Next() && !w.state.IsStopping() {
		rec := r.Record()
		w.metrics.recordsRead.WithLabelValues(w.id).Inc()
		read, err := w.decodeAndDispatch(rec, segmentNum)
		// keep true if data was read at least once
		readData = readData || read
		if err != nil {
			return readData, fmt.Errorf("error decoding record: %w", err)
		}
	}

	if r.Err() != nil {
		return readData, fmt.Errorf("segment %d: %w", segmentNum, r.Err())
	}

	return readData, nil
}

// decodeAndDispatch first decodes a WAL record. Upon reading either Series or Entries from the WAL record, call the
// appropriate callbacks in the writeTo.
func (w *Watcher) decodeAndDispatch(b []byte, segmentNum int) (bool, error) {
	var readData bool

	rec := recordPool.GetRecord()
	defer func() { recordPool.PutRecord(rec) }()

	if err := DecodeRecord(b, rec); err != nil {
		w.metrics.recordDecodeFails.WithLabelValues(w.id).Inc()
		return readData, err
	}

	// First process all series to ensure we don't write entries to non-existent series.
	var firstErr error
	w.actions.StoreSeries(rec.Series, segmentNum)
	readData = true

	for _, entries := range rec.RefEntries {
		if err := w.actions.AppendEntries(entries, segmentNum); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return readData, firstErr
}

// Drain moves the Watcher to a draining state, which will assume no more data is being written to the WAL, and it will
// attempt to read until the end of the last written segment. The calling routine of Drain will block until all data is
// read, or a timeout occurs.
func (w *Watcher) Drain() {
	w.logger.Info("draining Watcher")
	w.state.Transition(internal.StateDraining)
	// wait for drain timeout, or stopping state, in case the Watcher does the transition itself promptly
	select {
	case <-time.NewTimer(w.drainTimeout).C:
		w.logger.Warn("watcher drain timeout occurred, transitioning to Stopping")
	case <-w.state.WaitForStopping():
	}
}

// Stop stops the Watcher, shutting down the main routine.
func (w *Watcher) Stop() {
	w.state.Transition(internal.StateStopping)

	// upon calling stop, wait for main mainLoop execution to stop
	<-w.done

	w.metrics.watchersRunning.WithLabelValues().Dec()
}

// firstAndLast finds the first and last segment number for a WAL directory.
func (w *Watcher) firstAndLast() (int, int, error) { //nolint:unparam
	refs, err := readSegmentNumbers(w.walDir)
	if err != nil {
		return -1, -1, err
	}

	if len(refs) == 0 {
		return -1, -1, nil
	}

	// Start with sentinel values and walk back to the first and last (min and max)
	var first = math.MaxInt32
	var last = -1
	for _, segmentReg := range refs {
		if segmentReg < first {
			first = segmentReg
		}
		if segmentReg > last {
			last = segmentReg
		}
	}
	return first, last, nil
}

// NotifyWrite allows the Watcher to subscribe to write events published by the Writer. When a write event is received
// we emit the signal to trigger a segment read on the watcher main routine. If the readNotify channel already is not being
// listened on, that means the main routine is processing a segment,  or waiting because a non-handled error occurred.
// In that case we drop the signal and make the Watcher wait for the next one.
func (w *Watcher) NotifyWrite() {
	select {
	case w.readNotify <- struct{}{}:
		// written notification to the channel
		return
	default:
		// drop wal written signal if the channel is not being listened
		w.metrics.droppedWriteNotifications.WithLabelValues(w.id).Inc()
	}
}

// findNextSegmentFor finds the first segment greater than or equal to index.
func (w *Watcher) findNextSegmentFor(index int) (int, error) {
	// TODO(thepalbi): is segs in order?
	segs, err := readSegmentNumbers(w.walDir)
	if err != nil {
		return -1, err
	}

	for _, r := range segs {
		if r > index {
			return r, nil
		}
	}

	return -1, errors.New("failed to find segment for index")
}

// readSegmentNumbers reads the given directory and returns all segment identifiers, that is, the index of each segment
// file.
func readSegmentNumbers(dir string) ([]int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var refs []int
	for _, f := range files {
		k, err := strconv.Atoi(f.Name())
		if err != nil {
			continue
		}
		refs = append(refs, k)
	}
	return refs, nil
}

// Get size of segment.
func getSegmentSize(dir string, index int) (int64, error) {
	i := int64(-1)
	fi, err := os.Stat(wlog.SegmentName(dir, index))
	if err == nil {
		i = fi.Size()
	}
	return i, err
}
