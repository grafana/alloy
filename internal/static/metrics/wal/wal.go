package wal

// This code is copied from
// prometheus/prometheus@7c2de14b0bd74303c2ca6f932b71d4585a29ca75, with only
// minor changes for metric names.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
	"github.com/prometheus/prometheus/util/compression"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util"
)

// Upstream prometheus implementation https://github.com/prometheus/prometheus/blob/main/tsdb/agent/db.go
// 	based on the prometheus tsdb head wal https://github.com/prometheus/prometheus/blob/main/tsdb/head_wal.go

// ErrWALClosed is an error returned when a WAL operation can't run because the
// storage has already been closed.
var ErrWALClosed = fmt.Errorf("WAL storage closed")

type storageMetrics struct {
	r prometheus.Registerer

	numActiveSeries        prometheus.Gauge
	numDeletedSeries       prometheus.Gauge
	totalOutOfOrderSamples prometheus.Counter
	totalCreatedSeries     prometheus.Counter
	totalRemovedSeries     prometheus.Counter
	totalAppendedSamples   prometheus.Counter
	totalAppendedExemplars prometheus.Counter
	totalAppendedMetadata  prometheus.Counter
}

func newStorageMetrics(r prometheus.Registerer) *storageMetrics {
	m := storageMetrics{r: r}
	m.numActiveSeries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_remote_write_wal_storage_active_series",
		Help: "Current number of active series being tracked by the WAL storage",
	})

	m.numDeletedSeries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "prometheus_remote_write_wal_storage_deleted_series",
		Help: "Current number of series marked for deletion from memory",
	})

	m.totalOutOfOrderSamples = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_out_of_order_samples_total",
		Help: "Total number of out of order samples ingestion failed attempts.",
	})

	m.totalCreatedSeries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_storage_created_series_total",
		Help: "Total number of created series appended to the WAL",
	})

	m.totalRemovedSeries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_storage_removed_series_total",
		Help: "Total number of created series removed from the WAL",
	})

	m.totalAppendedSamples = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_samples_appended_total",
		Help: "Total number of samples appended to the WAL",
	})

	m.totalAppendedExemplars = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_exemplars_appended_total",
		Help: "Total number of exemplars appended to the WAL",
	})

	m.totalAppendedMetadata = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "prometheus_remote_write_wal_metadata_updates_total",
		Help: "Total number of metadata updates sent through the WAL",
	})

	if r != nil {
		m.numActiveSeries = util.MustRegisterOrGet(r, m.numActiveSeries).(prometheus.Gauge)
		m.numDeletedSeries = util.MustRegisterOrGet(r, m.numDeletedSeries).(prometheus.Gauge)
		m.totalOutOfOrderSamples = util.MustRegisterOrGet(r, m.totalOutOfOrderSamples).(prometheus.Counter)
		m.totalCreatedSeries = util.MustRegisterOrGet(r, m.totalCreatedSeries).(prometheus.Counter)
		m.totalRemovedSeries = util.MustRegisterOrGet(r, m.totalRemovedSeries).(prometheus.Counter)
		m.totalAppendedSamples = util.MustRegisterOrGet(r, m.totalAppendedSamples).(prometheus.Counter)
		m.totalAppendedExemplars = util.MustRegisterOrGet(r, m.totalAppendedExemplars).(prometheus.Counter)
		m.totalAppendedMetadata = util.MustRegisterOrGet(r, m.totalAppendedMetadata).(prometheus.Counter)
	}

	return &m
}

func (m *storageMetrics) Unregister() {
	if m.r == nil {
		return
	}
	cs := []prometheus.Collector{
		m.numActiveSeries,
		m.numDeletedSeries,
		m.totalOutOfOrderSamples,
		m.totalCreatedSeries,
		m.totalRemovedSeries,
		m.totalAppendedSamples,
		m.totalAppendedExemplars,
		m.totalAppendedMetadata,
	}
	for _, c := range cs {
		m.r.Unregister(c)
	}
}

// Storage implements storage.Storage, and just writes to the WAL.
type Storage struct {
	// Embed Queryable/ChunkQueryable for compatibility, but don't actually implement it.
	storage.Queryable
	storage.ChunkQueryable

	// Operations against the WAL must be protected by a mutex so it doesn't get
	// closed in the middle of an operation. Other operations are concurrency-safe, so we
	// use a RWMutex to allow multiple usages of the WAL at once. If the WAL is closed, all
	// operations that change the WAL must fail.
	walMtx    sync.RWMutex
	walClosed bool

	path   string
	wal    *wlog.WL
	logger log.Logger

	appenderPool sync.Pool
	bufPool      sync.Pool

	nextRef *atomic.Uint64
	series  *stripeSeries
	deleted map[chunks.HeadSeriesRef]int // Deleted series, and what WAL segment they must be kept until.

	metrics *storageMetrics

	notifier wlog.WriteNotified
}

// stripeSeriesSize is the number of stripes to use for series locking. A larger number allows for more concurrent writes without
// concerns for lock contention, but allocates a static amount of memory for each remote write component in use. This value is
// 4x smaller than the current default for prometheus TSDB. There are discussions about dropping it even further in,
// https://github.com/prometheus/prometheus/issues/17100. This value is likely still too high for our use case, but we
// cannot easily test for lock contention we will play it safe and keep it higher for now.
var stripeSeriesSize = 4096

// NewStorage makes a new Storage.
func NewStorage(logger log.Logger, registerer prometheus.Registerer, path string) (*Storage, error) {
	// Convert go-kit logger to slog logger
	slogLogger := slog.New(logging.NewSlogGoKitHandler(logger))

	w, err := wlog.NewSize(slogLogger, registerer, SubDirectory(path), wlog.DefaultSegmentSize, compression.Snappy)
	if err != nil {
		return nil, err
	}

	storage := &Storage{
		path:    path,
		wal:     w,
		logger:  logger,
		deleted: map[chunks.HeadSeriesRef]int{},
		series:  newStripeSeries(stripeSeriesSize),
		metrics: newStorageMetrics(registerer),
		nextRef: atomic.NewUint64(0),
	}

	storage.bufPool.New = func() any {
		b := make([]byte, 0, 1024)
		return b
	}

	storage.appenderPool.New = func() any {
		return &appender{
			w:                      storage,
			pendingSeries:          make([]record.RefSeries, 0, 100),
			pendingSamples:         make([]record.RefSample, 0, 100),
			pendingHistograms:      make([]record.RefHistogramSample, 0, 100),
			pendingFloatHistograms: make([]record.RefFloatHistogramSample, 0, 100),
			pendingExamplars:       make([]record.RefExemplar, 0, 10),
			pendingMetadata:        make([]record.RefMetadata, 0, 10),
		}
	}

	if err := storage.replayWAL(); err != nil {
		level.Warn(storage.logger).Log("msg", "encountered WAL read error, attempting repair", "err", err)

		var ce *wlog.CorruptionErr
		if ok := errors.As(err, &ce); !ok {
			return nil, err
		}
		if err := w.Repair(ce); err != nil {
			// if repair fails, truncate everything in WAL
			level.Warn(storage.logger).Log("msg", "WAL repair failed, truncating!", "err", err)
			if e := w.Truncate(math.MaxInt); e != nil {
				level.Error(storage.logger).Log("msg", "WAL truncate failure", "err", e)
				return nil, fmt.Errorf("truncate corrupted WAL: %w", e)
			}
			if e := wlog.DeleteCheckpoints(w.Dir(), math.MaxInt); e != nil {
				return nil, fmt.Errorf("delete WAL checkpoints: %w", e)
			}
			return nil, fmt.Errorf("repair corrupted WAL: %w", err)
		}
	}

	return storage, nil
}

func (w *Storage) replayWAL() error {
	w.walMtx.RLock()
	defer w.walMtx.RUnlock()

	if w.walClosed {
		return ErrWALClosed
	}

	level.Info(w.logger).Log("msg", "replaying WAL, this may take a while", "dir", w.wal.Dir())
	dir, startFrom, err := wlog.LastCheckpoint(w.wal.Dir())
	if err != nil && !errors.Is(err, record.ErrNotFound) {
		return fmt.Errorf("find last checkpoint: %w", err)
	}

	duplicateRefToValidRef := map[chunks.HeadSeriesRef]chunks.HeadSeriesRef{}
	if err == nil {
		sr, err := wlog.NewSegmentsReader(dir)
		if err != nil {
			return fmt.Errorf("open checkpoint: %w", err)
		}
		defer func() {
			if err := sr.Close(); err != nil {
				level.Warn(w.logger).Log("msg", "error while closing the wal segments reader", "err", err)
			}
		}()

		// A corrupted checkpoint is a hard error for now and requires user
		// intervention. There's likely little data that can be recovered anyway.
		if err := w.loadWAL(wlog.NewReader(sr), duplicateRefToValidRef, startFrom); err != nil {
			return fmt.Errorf("backfill checkpoint: %w", err)
		}
		startFrom++
		level.Info(w.logger).Log("msg", "WAL checkpoint loaded")
	}

	// Find the last segment.
	_, lastSegment, err := wlog.Segments(w.wal.Dir())
	if err != nil {
		return fmt.Errorf("finding WAL segments: %w", err)
	}

	// Backfill segments from the most recent checkpoint onwards.
	for i := startFrom; i <= lastSegment; i++ {
		s, err := wlog.OpenReadSegment(wlog.SegmentName(w.wal.Dir(), i))
		if err != nil {
			return fmt.Errorf("open WAL segment %d: %w", i, err)
		}

		sr := wlog.NewSegmentBufReader(s)
		err = w.loadWAL(wlog.NewReader(sr), duplicateRefToValidRef, i)
		if err := sr.Close(); err != nil {
			level.Warn(w.logger).Log("msg", "error while closing the wal segments reader", "err", err)
		}
		if err != nil {
			return err
		}
		level.Info(w.logger).Log("msg", "WAL segment loaded", "segment", i, "maxSegment", lastSegment)
	}

	return nil
}

// loadWAL reads the WAL and populates the in-memory series.
// duplicateRefToValidRef tracks SeriesRefs that are duplicates by their labels, and maps them to the valid SeriesRef
// that should be used instead. Duplicate SeriesRefs for the same labels can happen when a series is gc'ed from memory
// but has not been fully removed from the WAL via a wlog.Checkpoint yet.
func (w *Storage) loadWAL(r *wlog.Reader, duplicateRefToValidRef map[chunks.HeadSeriesRef]chunks.HeadSeriesRef, currentSegmentOrCheckpoint int) (err error) {
	var (
		dec     = record.NewDecoder(nil, slog.New(logging.NewSlogGoKitHandler(w.logger)))
		lastRef = chunks.HeadSeriesRef(w.nextRef.Load())

		decoded    = make(chan any, 10)
		errCh      = make(chan error, 1)
		seriesPool = sync.Pool{
			New: func() any {
				return []record.RefSeries{}
			},
		}
		samplesPool = sync.Pool{
			New: func() any {
				return []record.RefSample{}
			},
		}
		histogramsPool = sync.Pool{
			New: func() any {
				return []record.RefHistogramSample{}
			},
		}
		floatHistogramsPool = sync.Pool{
			New: func() any {
				return []record.RefFloatHistogramSample{}
			},
		}
	)

	go func() {
		defer close(decoded)
		for r.Next() {
			rec := r.Record()
			// TODO: Also decode metadata. This could be particularly useful when
			// calling UpdateMetadata on the first Commit after startup -
			// right now each call will be logged as if the metadata changed.
			switch dec.Type(rec) {
			case record.Series:
				series := seriesPool.Get().([]record.RefSeries)[:0]
				series, err = dec.Series(rec, series)
				if err != nil {
					errCh <- &wlog.CorruptionErr{
						Err:     fmt.Errorf("decode series: %w", err),
						Segment: r.Segment(),
						Offset:  r.Offset(),
					}
					return
				}
				decoded <- series
			case record.Samples:
				samples := samplesPool.Get().([]record.RefSample)[:0]
				samples, err = dec.Samples(rec, samples)
				if err != nil {
					errCh <- &wlog.CorruptionErr{
						Err:     fmt.Errorf("decode samples: %w", err),
						Segment: r.Segment(),
						Offset:  r.Offset(),
					}
				}
				decoded <- samples
			case record.HistogramSamples:
				histograms := histogramsPool.Get().([]record.RefHistogramSample)[:0]
				histograms, err = dec.HistogramSamples(rec, histograms)
				if err != nil {
					errCh <- &wlog.CorruptionErr{
						Err:     fmt.Errorf("decode histogram samples: %w", err),
						Segment: r.Segment(),
						Offset:  r.Offset(),
					}
					return
				}
				decoded <- histograms
			case record.FloatHistogramSamples:
				floatHistograms := floatHistogramsPool.Get().([]record.RefFloatHistogramSample)[:0]
				floatHistograms, err = dec.FloatHistogramSamples(rec, floatHistograms)
				if err != nil {
					errCh <- &wlog.CorruptionErr{
						Err:     fmt.Errorf("decode float histogram samples: %w", err),
						Segment: r.Segment(),
						Offset:  r.Offset(),
					}
					return
				}
				decoded <- floatHistograms
			case record.Tombstones, record.Exemplars:
				// We don't care about decoding tombstones or exemplars
				// TODO: If decide to decode exemplars, we should make sure to prepopulate
				// stripeSeries.exemplars in the next block by using setLatestExemplar.
				continue
			default:
				errCh <- &wlog.CorruptionErr{
					Err:     fmt.Errorf("invalid record type %v", dec.Type(rec)),
					Segment: r.Segment(),
					Offset:  r.Offset(),
				}
				return
			}
		}
	}()

	var nonExistentSeriesRefs atomic.Uint64

	for d := range decoded {
		switch v := d.(type) {
		case []record.RefSeries:
			for _, s := range v {
				// Make sure we don't try to reuse a Ref that already exists in the WAL.
				if s.Ref > lastRef {
					lastRef = s.Ref
				}

				series := &memSeries{ref: s.Ref, lset: s.Labels, lastTs: 0}
				series, created := w.series.GetOrSet(s.Labels.Hash(), s.Labels, series)
				if !created {
					duplicateRefToValidRef[s.Ref] = series.ref
					// Make sure we keep the duplicate SeriesRef checkpoints while it might still exist in the WAL.
					w.deleted[s.Ref] = currentSegmentOrCheckpoint
				} else {
					w.metrics.numActiveSeries.Inc()
					w.metrics.totalCreatedSeries.Inc()
				}
			}

			//nolint:staticcheck
			seriesPool.Put(v)
		case []record.RefSample:
			for _, s := range v {
				if ref, ok := duplicateRefToValidRef[s.Ref]; ok {
					// Make sure we keep the duplicate SeriesRef in checkpoints until we get past the current segment.
					w.deleted[s.Ref] = currentSegmentOrCheckpoint
					s.Ref = ref
				}
				series := w.series.GetByID(s.Ref)
				if series == nil {
					nonExistentSeriesRefs.Inc()
					continue
				}

				// Update the lastTs for the series if this sample is newer
				if s.T > series.lastTs {
					series.lastTs = s.T
				}
			}

			//nolint:staticcheck
			samplesPool.Put(v)
		case []record.RefHistogramSample:
			for _, entry := range v {
				if ref, ok := duplicateRefToValidRef[entry.Ref]; ok {
					entry.Ref = ref
				}
				series := w.series.GetByID(entry.Ref)
				if series == nil {
					nonExistentSeriesRefs.Inc()
					continue
				}

				// Update the lastTs for the series if this sample is newer
				if entry.T > series.lastTs {
					series.lastTs = entry.T
				}
			}

			//nolint:staticcheck
			histogramsPool.Put(v)
		case []record.RefFloatHistogramSample:
			for _, entry := range v {
				if ref, ok := duplicateRefToValidRef[entry.Ref]; ok {
					entry.Ref = ref
				}
				series := w.series.GetByID(entry.Ref)
				if series == nil {
					nonExistentSeriesRefs.Inc()
					continue
				}

				// Update the lastTs for the series if this sample is newer
				if entry.T > series.lastTs {
					series.lastTs = entry.T
				}
			}

			//nolint:staticcheck
			floatHistogramsPool.Put(v)
		default:
			panic(fmt.Errorf("unexpected decoded type: %T", d))
		}
	}

	if v := nonExistentSeriesRefs.Load(); v > 0 {
		level.Warn(w.logger).Log("msg", "found sample referencing non-existing series", "skipped_series", v)
	}

	w.nextRef.Store(uint64(lastRef))

	select {
	case err := <-errCh:
		return err
	default:
		if r.Err() != nil {
			return fmt.Errorf("read records: %w", r.Err())
		}
		return nil
	}
}

// SetNotifier sets the notifier for the WAL storage. SetNotifier must only be
// called before data is written to the WAL.
func (w *Storage) SetNotifier(n wlog.WriteNotified) {
	w.notifier = n
}

// Directory returns the path where the WAL storage is held.
func (w *Storage) Directory() string {
	return w.path
}

// Appender returns a new appender against the storage.
func (w *Storage) Appender(_ context.Context) storage.Appender {
	return w.appenderPool.Get().(storage.Appender)
}

// StartTime always returns 0, nil. It is implemented for compatibility with
// Prometheus, but is unused in the agent.
func (*Storage) StartTime() (int64, error) {
	return 0, nil
}

// Truncate removes all data from the WAL prior to the timestamp specified by
// mint.
func (w *Storage) Truncate(mint int64) error {
	w.walMtx.RLock()
	defer w.walMtx.RUnlock()

	if w.walClosed {
		return ErrWALClosed
	}

	start := time.Now()

	// Garbage collect series that haven't received an update since mint.
	w.gc(mint)
	level.Info(w.logger).Log("msg", "series GC completed", "duration", time.Since(start))

	first, last, err := wlog.Segments(w.wal.Dir())
	if err != nil {
		return fmt.Errorf("get segment range: %w", err)
	}

	// Start a new segment, so low ingestion volume instance don't have more WAL
	// than needed.
	_, err = w.wal.NextSegment()
	if err != nil {
		return fmt.Errorf("next segment: %w", err)
	}

	last-- // Never consider last segment for checkpoint.
	if last < 0 {
		return nil // no segments yet.
	}

	// The lower two thirds of segments should contain mostly obsolete samples.
	// If we have less than two segments, it's not worth checkpointing yet.
	last = first + (last-first)*2/3
	if last <= first {
		return nil
	}

	keep := func(id chunks.HeadSeriesRef) bool {
		if w.series.GetByID(id) != nil {
			return true
		}

		seg, ok := w.deleted[id]
		return ok && seg > last
	}

	// Convert go-kit logger to slog logger
	slogLogger := slog.New(logging.NewSlogGoKitHandler(w.logger))

	if _, err = wlog.Checkpoint(slogLogger, w.wal, first, last, keep, mint); err != nil {
		return fmt.Errorf("create checkpoint: %w", err)
	}
	if err := w.wal.Truncate(last + 1); err != nil {
		// If truncating fails, we'll just try again at the next checkpoint.
		// Leftover segments will just be ignored in the future if there's a checkpoint
		// that supersedes them.
		level.Error(w.logger).Log("msg", "truncating segments failed", "err", err)
	}

	// The checkpoint is written and segments before it is truncated, so we no
	// longer need to track deleted series that are before it.
	for ref, segment := range w.deleted {
		if segment <= last {
			delete(w.deleted, ref)
			w.metrics.totalRemovedSeries.Inc()
		}
	}
	w.metrics.numDeletedSeries.Set(float64(len(w.deleted)))

	if err := wlog.DeleteCheckpoints(w.wal.Dir(), last); err != nil {
		// Leftover old checkpoints do not cause problems down the line beyond
		// occupying disk space.
		// They will just be ignored since a higher checkpoint exists.
		level.Error(w.logger).Log("msg", "delete old checkpoints", "err", err)
	}

	level.Info(w.logger).Log("msg", "WAL checkpoint complete",
		"first", first, "last", last, "duration", time.Since(start))
	return nil
}

// gc removes data before the minimum timestamp from the head.
func (w *Storage) gc(mint int64) {
	deleted := w.series.gc(mint)
	w.metrics.numActiveSeries.Sub(float64(len(deleted)))

	_, last, _ := wlog.Segments(w.wal.Dir())

	// We want to keep series records for any newly deleted series
	// until we've passed the last recorded segment. This prevents
	// the WAL having samples for series records that no longer exist.
	for ref := range deleted {
		w.deleted[ref] = last
	}

	w.metrics.numDeletedSeries.Set(float64(len(w.deleted)))
}

// Close closes the storage and all its underlying resources.
func (w *Storage) Close() error {
	w.walMtx.Lock()
	defer w.walMtx.Unlock()

	if w.walClosed {
		return fmt.Errorf("already closed")
	}
	w.walClosed = true

	if w.metrics != nil {
		w.metrics.Unregister()
	}
	return w.wal.Close()
}

type appender struct {
	w                      *Storage
	pendingSeries          []record.RefSeries
	pendingSamples         []record.RefSample
	pendingExamplars       []record.RefExemplar
	pendingHistograms      []record.RefHistogramSample
	pendingFloatHistograms []record.RefFloatHistogramSample
	pendingMetadata        []record.RefMetadata

	// Pointers to the series referenced by each element of pendingSamples.
	// Series lock is not held on elements.
	sampleSeries []*memSeries

	// Pointers to the series referenced by each element of pendingHistograms.
	// Series lock is not held on elements.
	histogramSeries []*memSeries

	// Pointers to the series referenced by each element of pendingFloatHistograms.
	// Series lock is not held on elements.
	floatHistogramSeries []*memSeries

	// Pointers to the series referenced by each element of pendingMetadata.
	// Series lock is not held on elements.
	metadataSeries []*memSeries
}

var _ storage.Appender = (*appender)(nil)

func (a *appender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	series := a.w.series.GetByID(chunks.HeadSeriesRef(ref))
	if series == nil {
		// Ensure no empty or duplicate labels have gotten through. This mirrors the
		// equivalent validation code in the TSDB's headAppender.
		l = l.WithoutEmpty()
		if l.Len() == 0 {
			return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
		}

		if lbl, dup := l.HasDuplicateLabelNames(); dup {
			return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidSample)
		}

		var created bool
		series, created = a.getOrCreate(l)
		if created {
			a.pendingSeries = append(a.pendingSeries, record.RefSeries{
				Ref:    series.ref,
				Labels: l,
			})

			a.w.metrics.numActiveSeries.Inc()
			a.w.metrics.totalCreatedSeries.Inc()
		}
	}

	series.Lock()
	defer series.Unlock()

	// NOTE(rfratto): always modify pendingSamples and sampleSeries together.
	a.pendingSamples = append(a.pendingSamples, record.RefSample{
		Ref: series.ref,
		T:   t,
		V:   v,
	})
	a.sampleSeries = append(a.sampleSeries, series)

	a.w.metrics.totalAppendedSamples.Inc()
	return storage.SeriesRef(series.ref), nil
}

func (a *appender) getOrCreate(l labels.Labels) (series *memSeries, created bool) {
	hash := l.Hash()

	series = a.w.series.GetByHash(hash, l)
	if series != nil {
		return series, false
	}

	ref := chunks.HeadSeriesRef(a.w.nextRef.Inc())
	series = &memSeries{ref: ref, lset: l, lastTs: math.MinInt64}
	a.w.series.Set(l.Hash(), series)
	return series, true
}

func (a *appender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	readRef := chunks.HeadSeriesRef(ref)

	s := a.w.series.GetByID(readRef)
	if s == nil {
		return 0, fmt.Errorf("unknown series ref when trying to add exemplar: %d", readRef)
	}

	// Ensure no empty labels have gotten through.
	e.Labels = e.Labels.WithoutEmpty()

	if lbl, dup := e.Labels.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidExemplar)
	}

	// Exemplar label length does not include chars involved in text rendering such as quotes
	// equals sign, or commas. See definition of const ExemplarMaxLabelLength.
	labelSetLen := 0
	err := e.Labels.Validate(func(l labels.Label) error {
		labelSetLen += utf8.RuneCountInString(l.Name)
		labelSetLen += utf8.RuneCountInString(l.Value)

		if labelSetLen > exemplar.ExemplarMaxLabelSetLength {
			return storage.ErrExemplarLabelLength
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Check for duplicate vs last stored exemplar for this series, and discard those.
	// Otherwise, record the current exemplar as the latest.
	// Prometheus' TSDB returns 0 when encountering duplicates, so we do the same here.
	// TODO(@tpaschalis) The case of OOO exemplars is currently unique to
	// Native Histograms. Prometheus is tracking a (possibly different) fix to
	// this, we should revisit once they've settled on a solution.
	// https://github.com/prometheus/prometheus/issues/12971
	prevExemplar := a.w.series.GetLatestExemplar(s.ref)
	if prevExemplar != nil && (prevExemplar.Equals(e) || prevExemplar.Ts > e.Ts) {
		// Duplicate, don't return an error but don't accept the exemplar.
		return 0, nil
	}
	a.w.series.SetLatestExemplar(s.ref, &e)

	a.pendingExamplars = append(a.pendingExamplars, record.RefExemplar{
		Ref:    readRef,
		T:      e.Ts,
		V:      e.Value,
		Labels: e.Labels,
	})

	a.w.metrics.totalAppendedExemplars.Inc()
	return storage.SeriesRef(s.ref), nil
}

func (a *appender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if h != nil {
		if err := h.Validate(); err != nil {
			return 0, err
		}
	}

	if fh != nil {
		if err := fh.Validate(); err != nil {
			return 0, err
		}
	}

	series := a.w.series.GetByID(chunks.HeadSeriesRef(ref))
	if series == nil {
		// Ensure no empty or duplicate labels have gotten through. This mirrors the
		// equivalent validation code in the TSDB's headAppender.
		l = l.WithoutEmpty()
		if l.Len() == 0 {
			return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
		}

		if lbl, dup := l.HasDuplicateLabelNames(); dup {
			return 0, fmt.Errorf("label name %q is not unique: %w", lbl, tsdb.ErrInvalidSample)
		}

		var created bool
		series, created = a.getOrCreate(l)
		if created {
			a.pendingSeries = append(a.pendingSeries, record.RefSeries{
				Ref:    series.ref,
				Labels: l,
			})

			a.w.metrics.numActiveSeries.Inc()
			a.w.metrics.totalCreatedSeries.Inc()
		}
	}

	series.Lock()
	defer series.Unlock()

	switch {
	case h != nil:
		// NOTE(rfratto): always modify pendingHistograms and histogramSeries
		// together.
		a.pendingHistograms = append(a.pendingHistograms, record.RefHistogramSample{
			Ref: series.ref,
			T:   t,
			H:   h,
		})
		a.histogramSeries = append(a.histogramSeries, series)
	case fh != nil:
		// NOTE(rfratto): always modify pendingFloatHistograms and
		// floatHistogramSeries together.
		a.pendingFloatHistograms = append(a.pendingFloatHistograms, record.RefFloatHistogramSample{
			Ref: series.ref,
			T:   t,
			FH:  fh,
		})
		a.floatHistogramSeries = append(a.floatHistogramSeries, series)
	}

	a.w.metrics.totalAppendedSamples.Inc()
	return storage.SeriesRef(series.ref), nil
}

func (a *appender) AppendCTZeroSample(_ storage.SeriesRef, _ labels.Labels, _ int64, _ int64) (storage.SeriesRef, error) {
	// TODO(ptodev): implement this later
	return 0, nil
}

func (a *appender) UpdateMetadata(ref storage.SeriesRef, labels labels.Labels, meta metadata.Metadata) (storage.SeriesRef, error) {
	// Ensure the series exists by id or by labels before we try to update metadata.
	series := a.w.series.GetByID(chunks.HeadSeriesRef(ref))
	if series == nil {
		series = a.w.series.GetByHash(labels.Hash(), labels)
		if series != nil {
			ref = storage.SeriesRef(series.ref)
		}
	}
	if series == nil {
		return 0, fmt.Errorf("unknown series when trying to add metadata with SeriesRef: %d and labels: %s", ref, labels)
	}

	series.Lock()
	// TODO: If the WAL is empty, then on the very first commit
	// hasNewMetadata will always be true repeatedly for the same metadata.
	// This is because series.meta is still nil - it's only set after the first commit.
	// Fix this here and in the upstream Prometheus WAL.
	hasNewMetadata := series.meta == nil || *series.meta != meta
	series.Unlock()

	if hasNewMetadata {
		a.pendingMetadata = append(a.pendingMetadata, record.RefMetadata{
			Ref:  chunks.HeadSeriesRef(ref),
			Type: record.GetMetricType(meta.Type),
			Unit: meta.Unit,
			Help: meta.Help,
		})
		a.metadataSeries = append(a.metadataSeries, series)
		a.w.metrics.totalAppendedMetadata.Inc()
	}
	return ref, nil
}

func (a *appender) SetOptions(_ *storage.AppendOptions) {
	// TODO: implement this later
	// TODO: currently only opts.DiscardOutOfOrder is available as an option. It is not supported in Alloy.
}

func (a *appender) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	// TODO: implement this later
	return 0, nil
}

// Commit submits the collected samples and purges the batch.
func (a *appender) Commit() error {
	if err := a.log(); err != nil {
		return err
	}

	if a.w.notifier != nil {
		a.w.notifier.Notify()
	}

	a.clearData()
	a.w.appenderPool.Put(a)
	return nil
}

func (a *appender) log() error {
	a.w.walMtx.RLock()
	defer a.w.walMtx.RUnlock()

	if a.w.walClosed {
		return ErrWALClosed
	}

	var encoder record.Encoder
	buf := a.w.bufPool.Get().([]byte)
	defer func() {
		a.w.bufPool.Put(buf) //nolint:staticcheck
	}()

	if len(a.pendingSeries) > 0 {
		buf = encoder.Series(a.pendingSeries, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
	}

	// Metadata needs to be written after series but before samples so we have a valid series ref for the metadata data
	// and that series ref can be associated to samples for metadata.
	if len(a.pendingMetadata) > 0 {
		buf = encoder.Metadata(a.pendingMetadata, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
	}

	if len(a.pendingSamples) > 0 {
		buf = encoder.Samples(a.pendingSamples, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
	}

	if len(a.pendingHistograms) > 0 {
		var customBucketsHistograms []record.RefHistogramSample
		buf, customBucketsHistograms = encoder.HistogramSamples(a.pendingHistograms, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
		if len(customBucketsHistograms) > 0 {
			buf = encoder.CustomBucketsHistogramSamples(customBucketsHistograms, buf)
			if err := a.w.wal.Log(buf); err != nil {
				return fmt.Errorf("log custom buckets histograms: %w", err)
			}
			buf = buf[:0]
		}
	}

	if len(a.pendingFloatHistograms) > 0 {
		var customBucketsFloatHistograms []record.RefFloatHistogramSample
		buf, customBucketsFloatHistograms = encoder.FloatHistogramSamples(a.pendingFloatHistograms, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
		if len(customBucketsFloatHistograms) > 0 {
			buf = encoder.CustomBucketsFloatHistogramSamples(customBucketsFloatHistograms, buf)
			if err := a.w.wal.Log(buf); err != nil {
				return fmt.Errorf("log custom buckets histograms: %w", err)
			}
			buf = buf[:0]
		}
	}

	// Exemplars should be logged after samples (float/native histogram/etc),
	// otherwise it might happen that we send the exemplars in a remote write
	// batch before the samples, which in turn means the exemplar is rejected
	// for missing series, since series are created due to samples.
	if len(a.pendingExamplars) > 0 {
		buf = encoder.Exemplars(a.pendingExamplars, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
	}

	var series *memSeries
	for i, s := range a.pendingSamples {
		series = a.sampleSeries[i]
		if !series.updateTimestamp(s.T) {
			a.w.metrics.totalOutOfOrderSamples.Inc()
		}
	}
	for i, s := range a.pendingHistograms {
		series = a.histogramSeries[i]
		if !series.updateTimestamp(s.T) {
			a.w.metrics.totalOutOfOrderSamples.Inc()
		}
	}
	for i, s := range a.pendingFloatHistograms {
		series = a.floatHistogramSeries[i]
		if !series.updateTimestamp(s.T) {
			a.w.metrics.totalOutOfOrderSamples.Inc()
		}
	}
	for i, m := range a.pendingMetadata {
		series = a.metadataSeries[i]
		series.Lock()
		series.meta = &metadata.Metadata{Type: record.ToMetricType(m.Type), Unit: m.Unit, Help: m.Help}
		series.Unlock()
	}

	return nil
}

// clearData clears all pending data.
func (a *appender) clearData() {
	a.pendingSeries = a.pendingSeries[:0]
	a.pendingSamples = a.pendingSamples[:0]
	a.pendingHistograms = a.pendingHistograms[:0]
	a.pendingFloatHistograms = a.pendingFloatHistograms[:0]
	a.pendingExamplars = a.pendingExamplars[:0]
	a.pendingMetadata = a.pendingMetadata[:0]
	a.sampleSeries = a.sampleSeries[:0]
	a.histogramSeries = a.histogramSeries[:0]
	a.floatHistogramSeries = a.floatHistogramSeries[:0]
	a.metadataSeries = a.metadataSeries[:0]
}

func (a *appender) Rollback() error {
	// Series are created in-memory regardless of rollback. This means we must
	// log them to the WAL, otherwise subsequent commits may reference a series
	// which was never written to the WAL.
	if err := a.logSeries(); err != nil {
		return err
	}

	a.clearData()
	a.w.appenderPool.Put(a)
	return nil
}

// logSeries logs only pending series records to the WAL.
func (a *appender) logSeries() error {
	a.w.walMtx.RLock()
	defer a.w.walMtx.RUnlock()

	if a.w.walClosed {
		return ErrWALClosed
	}

	if len(a.pendingSeries) > 0 {
		var encoder record.Encoder
		buf := a.w.bufPool.Get().([]byte)
		defer func() {
			a.w.bufPool.Put(buf) //nolint:staticcheck
		}()

		buf = encoder.Series(a.pendingSeries, buf)
		if err := a.w.wal.Log(buf); err != nil {
			return err
		}
		buf = buf[:0]
	}

	return nil
}
