package wal

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func TestStorage_InvalidSeries(t *testing.T) {
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()

	app := s.Appender(t.Context())

	// Samples
	_, err = app.Append(0, labels.Labels{}, 0, 0)
	require.Error(t, err, "should reject empty labels")

	_, err = app.Append(0, labels.FromStrings("a", "1", "a", "2"), 0, 0)
	require.Error(t, err, "should reject duplicate labels")

	// Sanity check: valid series
	sRef, err := app.Append(0, labels.FromStrings("a", "1"), 0, 0)
	require.NoError(t, err, "should not reject valid series")

	// Exemplars
	_, err = app.AppendExemplar(0, labels.EmptyLabels(), exemplar.Exemplar{})
	require.Error(t, err, "should reject unknown series ref")

	e := exemplar.Exemplar{Labels: labels.FromStrings("a", "1", "a", "2")}
	_, err = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	require.ErrorIs(t, err, tsdb.ErrInvalidExemplar, "should reject duplicate labels")

	e = exemplar.Exemplar{Labels: labels.FromStrings("a_somewhat_long_trace_id", "nYJSNtFrFTY37VR7mHzEE/LIDt7cdAQcuOzFajgmLDAdBSRHYPDzrxhMA4zz7el8naI/AoXFv9/e/G0vcETcIoNUi3OieeLfaIRQci2oa")}
	_, err = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	require.ErrorIs(t, err, storage.ErrExemplarLabelLength, "should reject too long label length")

	// Sanity check: valid exemplars
	e = exemplar.Exemplar{Labels: labels.FromStrings("a", "1"), Value: 20, Ts: 10, HasTs: true}
	_, err = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	require.NoError(t, err, "should not reject valid exemplars")
}

func TestStorage(t *testing.T) {
	walDir := t.TempDir()

	onNotify := util.NewWaitTrigger()
	notifier := &fakeNotifier{NotitfyFunc: onNotify.Trigger}

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()
	s.SetNotifier(notifier)

	app := s.Appender(t.Context())

	// Write some samples
	payload := buildMixedTypeSeries()
	for _, metric := range payload {
		metric.Write(t, app)
	}

	require.NoError(t, app.Commit())

	collector := walDataCollector{}
	replayer := walReplayer{w: &collector}
	require.NoError(t, replayer.Replay(s.wal.Dir()))

	names := []string{}
	for _, series := range collector.series {
		names = append(names, series.Labels.Get("__name__"))
	}
	require.Equal(t, payload.SeriesNames(), names)

	expectedSamples, expectedHistograms, expectedFloatHistograms := payload.ExpectedSamples()
	actualSamples := collector.samples
	sort.Sort(byRefSample(actualSamples))
	require.Equal(t, expectedSamples, actualSamples)

	actualHistograms := collector.histograms
	sort.Sort(byRefHistogramSample(actualHistograms))
	require.Equal(t, expectedHistograms, actualHistograms)

	actualFloatHistograms := collector.floatHistograms
	sort.Sort(byRefFloatHistogramSample(actualFloatHistograms))
	require.Equal(t, expectedFloatHistograms, actualFloatHistograms)

	expectedExemplars := payload.ExpectedExemplars()
	actualExemplars := collector.exemplars
	sort.Sort(byRefExemplar(actualExemplars))
	require.Equal(t, expectedExemplars, actualExemplars)

	require.NoError(t, onNotify.Wait(time.Minute), "Expected Notify to be called")
}

func TestStorage_Rollback(t *testing.T) {
	walDir := t.TempDir()
	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, s.Close())
	})

	app := s.Appender(t.Context())

	payload := buildSeries([]string{"foo", "bar", "baz", "blerg"})
	for _, metric := range payload {
		metric.Write(t, app)
	}

	require.NoError(t, app.Rollback())

	var collector walDataCollector

	replayer := walReplayer{w: &collector}
	require.NoError(t, replayer.Replay(s.wal.Dir()))

	require.Len(t, collector.series, 4, "Series records should be written on Rollback")
	require.Len(t, collector.samples, 0, "Samples should not be written on rollback")
	require.Len(t, collector.exemplars, 0, "Exemplars should not be written on rollback")
	require.Len(t, collector.histograms, 0, "Histograms should not be written on rollback")
	require.Len(t, collector.floatHistograms, 0, "Native histograms should not be written on rollback")
}

func TestStorage_DuplicateExemplarsIgnored(t *testing.T) {
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()

	app := s.Appender(t.Context())

	sRef, err := app.Append(0, labels.FromStrings("a", "1"), 0, 0)
	require.NoError(t, err, "should not reject valid series")

	// If the Labels, Value or Timestamp are different than the last exemplar,
	// then a new one should be appended; Otherwise, it should be skipped.
	e := exemplar.Exemplar{Labels: labels.FromStrings("a", "1"), Value: 20, Ts: 10, HasTs: true}
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)

	e.Labels = labels.FromStrings("b", "2")
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)

	e.Value = 42
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)

	e.Ts = 25
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)

	e.Ts = 24
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)

	require.NoError(t, app.Commit())
	collector := walDataCollector{}
	replayer := walReplayer{w: &collector}
	require.NoError(t, replayer.Replay(s.wal.Dir()))

	// We had 11 calls to AppendExemplar but only 4 of those should have gotten through.
	require.Equal(t, 4, len(collector.exemplars))
}

func TestStorage_ExistingWAL(t *testing.T) {
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)

	app := s.Appender(t.Context())
	payload := buildSeries([]string{"foo", "bar", "baz", "blerg"})

	// Write half of the samples.
	for _, metric := range payload[0 : len(payload)/2] {
		metric.Write(t, app)
	}

	require.NoError(t, app.Commit())
	require.NoError(t, s.Close())

	// We need to wait a little bit for the previous store to finish
	// flushing.
	time.Sleep(time.Millisecond * 150)

	// Create a new storage, write the other half of samples.
	s, err = NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()

	// Verify that the storage picked up existing series when it
	// replayed the WAL.
	for series := range s.series.iterator().Channel() {
		require.Greater(t, series.lastTs, int64(0), "series timestamp not updated")
	}

	app = s.Appender(t.Context())

	for _, metric := range payload[len(payload)/2:] {
		metric.Write(t, app)
	}

	require.NoError(t, app.Commit())

	collector := walDataCollector{}
	replayer := walReplayer{w: &collector}
	require.NoError(t, replayer.Replay(s.wal.Dir()))

	names := []string{}
	for _, series := range collector.series {
		names = append(names, series.Labels.Get("__name__"))
	}
	require.Equal(t, payload.SeriesNames(), names)

	expectedSamples, _, _ := payload.ExpectedSamples()
	actualSamples := collector.samples
	sort.Sort(byRefSample(actualSamples))
	require.Equal(t, expectedSamples, actualSamples)

	expectedExemplars := payload.ExpectedExemplars()
	actualExemplars := collector.exemplars
	sort.Sort(byRefExemplar(actualExemplars))
	require.Equal(t, expectedExemplars, actualExemplars)
}

func TestStorage_ExistingWAL_RefID(t *testing.T) {
	l := util.TestLogger(t)

	walDir := t.TempDir()

	s, err := NewStorage(l, nil, walDir)
	require.NoError(t, err)

	app := s.Appender(t.Context())
	payload := buildSeries([]string{"foo", "bar", "baz", "blerg"})

	// Write all the samples
	for _, metric := range payload {
		metric.Write(t, app)
	}
	require.NoError(t, app.Commit())

	// Truncate the WAL to force creation of a new segment.
	require.NoError(t, s.Truncate(0))
	require.NoError(t, s.Close())

	// Create a new storage and see what the ref ID is initialized to.
	s, err = NewStorage(l, nil, walDir)
	require.NoError(t, err)
	defer require.NoError(t, s.Close())

	require.Equal(t, uint64(len(payload)), s.nextRef.Load(), "cached ref ID should be equal to the number of series written")
}

func TestStorage_Truncate(t *testing.T) {
	// Same as before but now do the following:
	// after writing all the data, forcefully create 4 more segments,
	// then do a truncate of a timestamp for _some_ of the data.
	// then read data back in. Expect to only get the latter half of data.
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()

	app := s.Appender(t.Context())

	payload := buildSeries([]string{"foo", "bar", "baz", "blerg"})

	for _, metric := range payload {
		metric.Write(t, app)
	}

	require.NoError(t, app.Commit())

	// Forcefully create a bunch of new segments so when we truncate
	// there's enough segments to be considered for truncation.
	for i := 0; i < 5; i++ {
		_, err := s.wal.NextSegmentSync()
		require.NoError(t, err)
	}

	// Truncate half of the samples, keeping only the second sample
	// per series.
	keepTs := payload[len(payload)-1].samples[0].ts + 1
	err = s.Truncate(keepTs)
	require.NoError(t, err)

	payload = payload.Filter(func(s sample) bool {
		return s.ts >= keepTs
	}, func(e exemplar.Exemplar) bool {
		return e.HasTs && e.Ts >= keepTs
	})
	expectedSamples, _, _ := payload.ExpectedSamples()
	expectedExemplars := payload.ExpectedExemplars()

	// Read back the WAL, collect series and samples.
	collector := walDataCollector{}
	replayer := walReplayer{w: &collector}
	require.NoError(t, replayer.Replay(s.wal.Dir()))

	names := []string{}
	for _, series := range collector.series {
		names = append(names, series.Labels.Get("__name__"))
	}
	require.Equal(t, payload.SeriesNames(), names)

	actualSamples := collector.samples
	sort.Sort(byRefSample(actualSamples))
	require.Equal(t, expectedSamples, actualSamples)

	actualExemplars := collector.exemplars
	sort.Sort(byRefExemplar(actualExemplars))
	require.Equal(t, expectedExemplars, actualExemplars)
}

func TestStorage_HandlesDuplicateSeriesRefsByHash(t *testing.T) {
	// Ensure the WAL can handle duplicate SeriesRefs by hash when being loaded.
	walDir := t.TempDir()

	s, err := NewStorage(log.NewLogfmtLogger(os.Stdout), nil, walDir)
	require.NoError(t, err)

	app := s.Appender(t.Context())

	var payload seriesList
	for i, metricName := range []string{"foo", "bar", "baz", "blerg"} {
		payload = append(payload, &series{
			name: metricName,
			samples: []sample{
				{int64(i), float64(i * 10.0), nil, nil},
				{int64(i * 10), float64(i * 100.0), nil, nil},
			},
		})
	}

	originalSeriesRefs := make([]chunks.HeadSeriesRef, 0, len(payload))
	for _, metric := range payload {
		metric.Write(t, app)
		originalSeriesRefs = append(originalSeriesRefs, chunks.HeadSeriesRef(*metric.ref))
	}
	require.NoError(t, app.Commit())

	// Forcefully create a bunch of new segments so when we truncate
	// there's enough segments to be considered for truncation.
	for i := 0; i < 3; i++ {
		_, err := s.wal.NextSegmentSync()
		require.NoError(t, err)
	}
	// Series are still active
	require.Equal(t, 4.0, testutil.ToFloat64(s.metrics.numActiveSeries))

	// Force GC of all the series, but they will stay in the checkpoint
	keepTs := payload[len(payload)-1].samples[1].ts + 1
	err = s.Truncate(keepTs)
	require.NoError(t, err)
	// No more active series because they were GC'ed with Truncate
	require.Equal(t, 0.0, testutil.ToFloat64(s.metrics.numActiveSeries))

	// Publish new samples that will create new SeriesRefs for the same labels.
	duplicateSeriesRefs := make([]chunks.HeadSeriesRef, 0, len(payload))
	for _, metric := range payload {
		metric.samples = metric.samples[1:]
		metric.samples[0].ts = metric.samples[0].ts * 10
		metric.Write(t, app)

		duplicateSeriesRefs = append(duplicateSeriesRefs, chunks.HeadSeriesRef(*metric.ref))
	}
	require.NoError(t, app.Commit())
	// We should be back to 4 active series now
	require.Equal(t, 4.0, testutil.ToFloat64(s.metrics.numActiveSeries))

	// Close the WAL before we have a chance to remove the first RefIDs
	require.NoError(t, s.Close())

	s, err = NewStorage(log.NewLogfmtLogger(os.Stdout), nil, walDir)
	require.NoError(t, err)

	// There should only be 4 active series after we reload the WAL
	assert.Equal(t, 4.0, testutil.ToFloat64(s.metrics.numActiveSeries))
	// The original SeriesRefs should be in series
	for _, ref := range originalSeriesRefs {
		assert.NotNil(t, s.series.GetByID(ref))
	}

	// The duplicated SeriesRefs should be considered deleted
	for _, ref := range duplicateSeriesRefs {
		assert.Contains(t, s.deleted, ref)
	}

	require.NoError(t, s.Close())
}

func TestStorage_WriteStalenessMarkers(t *testing.T) {
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()

	app := s.Appender(t.Context())

	// Write some samples
	payload := seriesList{
		{name: "foo", samples: []sample{{1, 10.0, nil, nil}, {10, 100.0, nil, nil}}},
		{name: "bar", samples: []sample{{2, 20.0, nil, nil}, {20, 200.0, nil, nil}}},
		{name: "baz", samples: []sample{{3, 30.0, nil, nil}, {30, 300.0, nil, nil}}},
	}
	for _, metric := range payload {
		metric.Write(t, app)
	}

	require.NoError(t, app.Commit())

	// Write staleness markers for every series
	require.NoError(t, s.WriteStalenessMarkers(func() int64 {
		// Pass math.MaxInt64 so it seems like everything was written already
		return math.MaxInt64
	}))

	// Read back the WAL, collect series and samples.
	collector := walDataCollector{}
	replayer := walReplayer{w: &collector}
	require.NoError(t, replayer.Replay(s.wal.Dir()))

	actual := collector.samples
	sort.Sort(byRefSample(actual))

	staleMap := map[chunks.HeadSeriesRef]bool{}
	for _, sample := range actual {
		if _, ok := staleMap[sample.Ref]; !ok {
			staleMap[sample.Ref] = false
		}
		if value.IsStaleNaN(sample.V) {
			staleMap[sample.Ref] = true
		}
	}

	for ref, v := range staleMap {
		require.True(t, v, "ref %d doesn't have stale marker", ref)
	}
}

func TestStorage_TruncateAfterClose(t *testing.T) {
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)

	require.NoError(t, s.Close())
	require.Error(t, ErrWALClosed, s.Truncate(0))
}

func TestStorage_Corruption(t *testing.T) {
	walDir := t.TempDir()

	// Write a corrupt segment
	err := os.Mkdir(filepath.Join(walDir, "wal"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(walDir, "wal", "00000000"), []byte("hello world"), 0644)
	require.NoError(t, err)

	// The storage should be initialized correctly anyway.
	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Close())
}

func TestGlobalReferenceID_Normal(t *testing.T) {
	walDir := t.TempDir()

	s, _ := NewStorage(log.NewNopLogger(), nil, walDir)
	defer s.Close()
	app := s.Appender(t.Context())
	l := labels.New(labels.Label{
		Name:  "__name__",
		Value: "label1",
	})
	ref, err := app.Append(0, l, time.Now().UnixMilli(), 0.1)
	_ = app.Commit()
	require.NoError(t, err)
	require.True(t, ref == 1)
	ref2, err := app.Append(0, l, time.Now().UnixMilli(), 0.1)
	require.NoError(t, err)
	require.True(t, ref2 == 1)

	l2 := labels.New(labels.Label{
		Name:  "__name__",
		Value: "label2",
	})
	ref3, err := app.Append(0, l2, time.Now().UnixMilli(), 0.1)
	require.NoError(t, err)
	require.True(t, ref3 == 2)
}

func TestDBAllowOOOSamples(t *testing.T) {
	walDir := t.TempDir()

	s, err := NewStorage(log.NewNopLogger(), nil, walDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Close())
	}()

	app := s.Appender(t.Context())

	// Write some samples
	payload := buildSeries([]string{"foo", "bar", "baz"})
	for _, metric := range payload {
		metric.Write(t, app)
	}

	require.NoError(t, app.Commit())

	for _, metric := range payload {
		// We want to set the timestamp to before using this offset.
		// This should no longer trigger an out of order.
		metric.WriteOOO(t, app, 10_000)
	}
}

func BenchmarkAppendExemplar(b *testing.B) {
	walDir := b.TempDir()

	s, _ := NewStorage(log.NewNopLogger(), nil, walDir)
	defer s.Close()
	app := s.Appender(b.Context())
	sRef, _ := app.Append(0, labels.FromStrings("a", "1"), 0, 0)
	e := exemplar.Exemplar{Labels: labels.FromStrings("a", "1"), Value: 20, Ts: 10, HasTs: true}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e.Ts = int64(i)
		_, _ = app.AppendExemplar(sRef, labels.EmptyLabels(), e)
	}
	b.StopTimer()

	// Actually use appended exemplars in case they get eliminated
	_ = app.Commit()
}

func BenchmarkCreateSeries(b *testing.B) {
	walDir := b.TempDir()

	s, _ := NewStorage(log.NewNopLogger(), nil, walDir)
	defer s.Close()

	app := s.Appender(b.Context()).(*appender)
	lbls := make([]labels.Labels, b.N)

	for i, l := range labelsForTest("benchmark", b.N) {
		lbls[i] = labels.New(l...)
	}

	b.ResetTimer()

	for _, l := range lbls {
		app.getOrCreate(l)
	}
}

// Create series for tests.
func labelsForTest(lName string, seriesCount int) [][]labels.Label {
	var s [][]labels.Label

	for i := 0; i < seriesCount; i++ {
		lset := []labels.Label{
			{Name: "a", Value: lName},
			{Name: "instance", Value: "localhost" + strconv.Itoa(i)},
			{Name: "job", Value: "prometheus"},
		}
		s = append(s, lset)
	}

	return s
}

func BenchmarkStripeSeriesSize(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		newStripeSeries(stripeSeriesSize)
	}
}

// Type is float histograms if fh!=nil,
// otherwise integer histogram if h!=nil,
// otherwise float.
type sample struct {
	ts  int64
	val float64
	h   *histogram.Histogram
	fh  *histogram.FloatHistogram
}

type series struct {
	name      string
	samples   []sample
	exemplars []exemplar.Exemplar

	ref *storage.SeriesRef
}

func (s *series) Write(t *testing.T, app storage.Appender) {
	t.Helper()

	lbls := labels.FromMap(map[string]string{"__name__": s.name})

	appendFunc := func(ref storage.SeriesRef, s sample) (storage.SeriesRef, error) {
		if s.h != nil || s.fh != nil {
			return app.AppendHistogram(ref, lbls, s.ts, s.h, s.fh)
		}
		return app.Append(ref, lbls, s.ts, s.val)
	}

	offset := 0
	if s.ref == nil {
		// Write first sample to get ref ID
		ref, err := appendFunc(0, s.samples[0])
		require.NoError(t, err)

		s.ref = &ref
		offset = 1
	}

	// Write other data points with AddFast
	for _, sample := range s.samples[offset:] {
		ref, err := appendFunc(*s.ref, sample)
		// The ref we had changed stop using the old value
		if *s.ref != ref {
			s.ref = &ref
		}
		require.NoError(t, err)
	}

	sRef := *s.ref
	for _, exemplar := range s.exemplars {
		var err error
		sRef, err = app.AppendExemplar(sRef, labels.EmptyLabels(), exemplar)
		require.NoError(t, err)
	}
}

func (s *series) WriteOOO(t *testing.T, app storage.Appender, tsOffset int64) {
	t.Helper()

	lbls := labels.FromMap(map[string]string{"__name__": s.name})

	offset := 0
	if s.ref == nil {
		// Write first sample to get ref ID
		ref, err := app.Append(0, lbls, s.samples[0].ts-tsOffset, s.samples[0].val)
		require.NoError(t, err)

		s.ref = &ref
		offset = 1
	}

	// Write other data points with AddFast
	for _, sample := range s.samples[offset:] {
		_, err := app.Append(*s.ref, lbls, sample.ts-tsOffset, sample.val)
		require.NoError(t, err)
	}
}

type seriesList []*series

// Filter creates a new seriesList with series filtered by a sample
// keep predicate function.
func (s seriesList) Filter(fn func(s sample) bool, fnExemplar func(e exemplar.Exemplar) bool) seriesList {
	var ret seriesList

	for _, entry := range s {
		var (
			samples   []sample
			exemplars []exemplar.Exemplar
		)

		for _, sample := range entry.samples {
			if fn(sample) {
				samples = append(samples, sample)
			}
		}

		for _, e := range entry.exemplars {
			if fnExemplar(e) {
				exemplars = append(exemplars, e)
			}
		}

		if len(samples) > 0 && len(exemplars) > 0 {
			ret = append(ret, &series{
				name:      entry.name,
				ref:       entry.ref,
				samples:   samples,
				exemplars: exemplars,
			})
		}
	}

	return ret
}

func (s seriesList) SeriesNames() []string {
	names := make([]string, 0, len(s))
	for _, series := range s {
		names = append(names, series.name)
	}
	return names
}

// ExpectedSamples returns the list of expected samples, sorted by ref ID and timestamp
func (s seriesList) ExpectedSamples() (expect []record.RefSample, expectHistogram []record.RefHistogramSample, expectFloatHistogram []record.RefFloatHistogramSample) {
	for _, series := range s {
		for _, sample := range series.samples {
			switch {
			case sample.fh != nil:
				expectFloatHistogram = append(expectFloatHistogram, record.RefFloatHistogramSample{
					Ref: chunks.HeadSeriesRef(*series.ref),
					T:   sample.ts,
					FH:  sample.fh,
				})
			case sample.h != nil:
				expectHistogram = append(expectHistogram, record.RefHistogramSample{
					Ref: chunks.HeadSeriesRef(*series.ref),
					T:   sample.ts,
					H:   sample.h,
				})
			default:
				expect = append(expect, record.RefSample{
					Ref: chunks.HeadSeriesRef(*series.ref),
					T:   sample.ts,
					V:   sample.val,
				})
			}
		}
	}
	sort.Sort(byRefSample(expect))
	sort.Sort(byRefHistogramSample(expectHistogram))
	sort.Sort(byRefFloatHistogramSample(expectFloatHistogram))
	return expect, expectHistogram, expectFloatHistogram
}

// ExpectedExemplars returns the list of expected exemplars, sorted by ref ID and timestamp
func (s seriesList) ExpectedExemplars() []record.RefExemplar {
	expect := []record.RefExemplar{}
	for _, series := range s {
		for _, exemplar := range series.exemplars {
			expect = append(expect, record.RefExemplar{
				Ref:    chunks.HeadSeriesRef(*series.ref),
				T:      exemplar.Ts,
				V:      exemplar.Value,
				Labels: exemplar.Labels,
			})
		}
	}
	sort.Sort(byRefExemplar(expect))
	return expect
}

func buildSeries(nameSlice []string) seriesList {
	s := make(seriesList, 0, len(nameSlice))
	for i, n := range nameSlice {
		i++
		s = append(s, &series{
			name:    n,
			samples: []sample{{int64(i), float64(i * 10.0), nil, nil}, {int64(i * 10), float64(i * 100.0), nil, nil}},
			exemplars: []exemplar.Exemplar{
				{Labels: labels.FromStrings("foobar", "barfoo"), Value: float64(i * 10.0), Ts: int64(i), HasTs: true},
				{Labels: labels.FromStrings("lorem", "ipsum"), Value: float64(i * 100.0), Ts: int64(i * 10), HasTs: true},
			},
		})
	}
	return s
}

func buildMixedTypeSeries() seriesList {
	return seriesList{
		{
			name:    "float_series",
			samples: []sample{{1, 10.0, nil, nil}, {10, 100.0, nil, nil}},
			exemplars: []exemplar.Exemplar{
				{Labels: labels.FromStrings("foobar", "barfoo"), Value: float64(10.0), Ts: int64(1), HasTs: true},
				{Labels: labels.FromStrings("lorem", "ipsum"), Value: float64(100.0), Ts: int64(10), HasTs: true},
			},
		},
		// From now on I put -1 into the float to be different from default 0.
		// Since we should be reading back the histograms, we should never see
		// the -1.
		{
			name:    "integer histogram",
			samples: []sample{{2, -1, tsdbutil.GenerateTestHistogram(1), nil}, {20, -1, tsdbutil.GenerateTestHistogram(100), nil}},
			exemplars: []exemplar.Exemplar{
				{Labels: labels.FromStrings("foobar", "barfoo"), Value: float64(10.0), Ts: int64(2), HasTs: true},
				{Labels: labels.FromStrings("lorem", "ipsum"), Value: float64(100.0), Ts: int64(20), HasTs: true},
			},
		},
		{
			name:    "float histogram",
			samples: []sample{{3, -1, nil, tsdbutil.GenerateTestFloatHistogram(1)}, {30, -1, nil, tsdbutil.GenerateTestFloatHistogram(100)}},
			exemplars: []exemplar.Exemplar{
				{Labels: labels.FromStrings("foobar", "barfoo"), Value: float64(10.0), Ts: int64(3), HasTs: true},
				{Labels: labels.FromStrings("lorem", "ipsum"), Value: float64(100.0), Ts: int64(30), HasTs: true},
			},
		},
		{
			name:    "integer NHCB",
			samples: []sample{{2, -1, tsdbutil.GenerateTestCustomBucketsHistogram(1), nil}, {20, -1, tsdbutil.GenerateTestCustomBucketsHistogram(100), nil}},
			exemplars: []exemplar.Exemplar{
				{Labels: labels.FromStrings("foobar", "barfoo"), Value: float64(10.0), Ts: int64(2), HasTs: true},
				{Labels: labels.FromStrings("lorem", "ipsum"), Value: float64(100.0), Ts: int64(20), HasTs: true},
			},
		},
		{
			name:    "float NHCB",
			samples: []sample{{3, -1, nil, tsdbutil.GenerateTestCustomBucketsFloatHistogram(1)}, {30, -1, nil, tsdbutil.GenerateTestCustomBucketsFloatHistogram(100)}},
			exemplars: []exemplar.Exemplar{
				{Labels: labels.FromStrings("foobar", "barfoo"), Value: float64(10.0), Ts: int64(3), HasTs: true},
				{Labels: labels.FromStrings("lorem", "ipsum"), Value: float64(100.0), Ts: int64(30), HasTs: true},
			},
		},
	}
}

type byRefSample []record.RefSample

func (b byRefSample) Len() int      { return len(b) }
func (b byRefSample) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byRefSample) Less(i, j int) bool {
	if b[i].Ref == b[j].Ref {
		return b[i].T < b[j].T
	}
	return b[i].Ref < b[j].Ref
}

type byRefHistogramSample []record.RefHistogramSample

func (b byRefHistogramSample) Len() int      { return len(b) }
func (b byRefHistogramSample) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byRefHistogramSample) Less(i, j int) bool {
	if b[i].Ref == b[j].Ref {
		return b[i].T < b[j].T
	}
	return b[i].Ref < b[j].Ref
}

type byRefFloatHistogramSample []record.RefFloatHistogramSample

func (b byRefFloatHistogramSample) Len() int      { return len(b) }
func (b byRefFloatHistogramSample) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byRefFloatHistogramSample) Less(i, j int) bool {
	if b[i].Ref == b[j].Ref {
		return b[i].T < b[j].T
	}
	return b[i].Ref < b[j].Ref
}

type byRefExemplar []record.RefExemplar

func (b byRefExemplar) Len() int      { return len(b) }
func (b byRefExemplar) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byRefExemplar) Less(i, j int) bool {
	if b[i].Ref == b[j].Ref {
		return b[i].T < b[j].T
	}
	return b[i].Ref < b[j].Ref
}

type fakeNotifier struct {
	NotitfyFunc func()
}

func (fn *fakeNotifier) Notify() {
	if fn.NotitfyFunc != nil {
		fn.NotitfyFunc()
	}
}
