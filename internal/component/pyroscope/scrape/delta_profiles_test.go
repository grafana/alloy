package scrape

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	googlev1 "github.com/grafana/pyroscope/api/gen/proto/go/google/v1"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/klauspost/compress/gzip"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestDeltaProfilerAppender(t *testing.T) {

	lbs := labels.FromStrings(model.MetricNameLabel, pprofMemory)

	outSamples := []*pyroscope.RawSample{}
	appender := NewDeltaAppender(
		pyroscope.AppendableFunc(func(ctx context.Context, lbs labels.Labels, samples []*pyroscope.RawSample) error {
			outSamples = append(outSamples, samples...)
			// We expect all samples to have the delta label set to false so that the server won't do the delta again.
			require.Equal(t, "false", lbs.Get(pyroscope.LabelNameDelta))
			return nil
		}), lbs)

	// first sample (not compressed) should be dropped
	first := newMemoryProfile(0, (15 * time.Second).Nanoseconds())
	err := appender.Append(t.Context(), lbs, []*pyroscope.RawSample{{RawProfile: marshal(t, first)}})
	require.NoError(t, err)
	require.Len(t, outSamples, 0)

	second := newMemoryProfile(int64(15*time.Second), (15 * time.Second).Nanoseconds())
	second.Sample[0].Value[0] = 10

	// second sample (compressed) should compute the diff with the first one for the correct samples.
	err = appender.Append(t.Context(), lbs, []*pyroscope.RawSample{{RawProfile: compress(t, marshal(t, second))}})
	require.NoError(t, err)
	require.Len(t, outSamples, 1)

	expected := newMemoryProfile((15 * time.Second).Nanoseconds(), (15 * time.Second).Nanoseconds())
	expected.Sample[0].Value[0] = second.Sample[0].Value[0] - first.Sample[0].Value[0]
	expected.Sample[0].Value[1] = second.Sample[0].Value[1] - first.Sample[0].Value[1]

	actual := unmarshalCompressed(t, outSamples[0].RawProfile)
	require.Equal(t, expected, actual)
}

func TestDeltaProfilerAppenderNoop(t *testing.T) {
	actual := []*pyroscope.RawSample{}
	appender := NewDeltaAppender(
		pyroscope.AppendableFunc(func(ctx context.Context, lbs labels.Labels, samples []*pyroscope.RawSample) error {
			actual = append(actual, samples...)
			return nil
		}), labels.EmptyLabels())
	in := newMemoryProfile(0, 0)
	err := appender.Append(t.Context(), labels.EmptyLabels(), []*pyroscope.RawSample{{RawProfile: marshal(t, in)}})
	require.NoError(t, err)
	require.Len(t, actual, 1)
	require.Equal(t, in, unmarshal(t, actual[0].RawProfile))
}

func marshal(t testing.TB, profile *googlev1.Profile) []byte {
	t.Helper()
	data, err := profile.MarshalVT()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func compress(t testing.TB, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func unmarshalCompressed(t testing.TB, data []byte) *googlev1.Profile {
	t.Helper()
	var gzr gzip.Reader
	if err := gzr.Reset(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	defer gzr.Close()
	uncompressed, err := io.ReadAll(&gzr)
	if err != nil {
		t.Fatal(err)
	}
	result := &googlev1.Profile{}
	if err := result.UnmarshalVT(uncompressed); err != nil {
		t.Fatal(err)
	}
	return result
}

func unmarshal(t testing.TB, data []byte) *googlev1.Profile {
	t.Helper()
	result := &googlev1.Profile{}
	if err := result.UnmarshalVT(data); err != nil {
		t.Fatal(err)
	}
	return result
}

func newMemoryProfile(timeNano int64, durationNano int64) *googlev1.Profile {
	st := make(stringTable)
	profile := &googlev1.Profile{
		SampleType: []*googlev1.ValueType{
			{Type: st.addString("alloc_objects"), Unit: st.addString("count")},
			{Type: st.addString("alloc_space"), Unit: st.addString("bytes")},
			{Type: st.addString("inuse_objects"), Unit: st.addString("count")},
			{Type: st.addString("inuse_space"), Unit: st.addString("bytes")},
		},
		Mapping: []*googlev1.Mapping{
			{Id: 1, Filename: st.addString("foo.go"), HasFunctions: true},
		},
		Sample: []*googlev1.Sample{
			{LocationId: []uint64{1}, Value: []int64{1, 2, 3, 4}},
		},
		Location: []*googlev1.Location{
			{
				Id: 1, MappingId: 1, Line: []*googlev1.Line{
					{FunctionId: 1, Line: 1},
					{FunctionId: 2, Line: 1},
				},
			},
		},
		Function: []*googlev1.Function{
			{Id: 1, Name: st.addString("foo")},
			{Id: 2, Name: st.addString("bar")},
		},
		TimeNanos:         timeNano,
		DurationNanos:     durationNano,
		DefaultSampleType: 0,
	}
	profile.StringTable = st.table()
	return profile
}

type stringTable map[string]int

func (strings stringTable) table() []string {
	table := make([]string, len(strings))
	for s, i := range strings {
		table[i] = s
	}
	return table
}

func (strings stringTable) addString(s string) int64 {
	if len(strings) == 0 {
		strings[""] = 0
	}
	i, ok := strings[s]
	if !ok {
		i = len(strings)
		strings[s] = i
	}
	return int64(i)
}

var out []byte

func BenchmarkComputeDelta(b *testing.B) {
	data := compress(b, marshal(b, newMemoryProfile(1, 1)))
	app := newDeltaAppender(nil, nil)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		res, err := app.computeDelta(data)
		require.NoError(b, err)
		out = res
	}
}
