package relabel

import (
	"bytes"
	"context"
	"sync"
	"testing"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/google/pprof/profile"
	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestAppendRelabelsAndDropsSamples(t *testing.T) {
	app := newCaptureAppender()
	c := newComponent(t, app, []*alloy_relabel.Config{
		{
			SourceLabels: []string{"env"},
			TargetLabel:  "environment",
			Action:       "replace",
			Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
			Replacement:  "$1",
		},
		{
			Action: "labeldrop",
			Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("^env$")},
		},
		{
			SourceLabels: []string{"environment"},
			Action:       "drop",
			Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("^dev$")},
		},
	})

	in := buildProfile(t,
		map[string][]string{"env": {"prod"}},
		map[string][]string{"env": {"dev"}},
	)

	err := c.Append(t.Context(), labels.FromStrings("service_name", "svc"), []*pyroscope.RawSample{{ID: "1", RawProfile: in}})
	require.NoError(t, err)

	rawSamples := app.RawSamples()
	require.Len(t, rawSamples, 1)

	out := parseProfile(t, rawSamples[0].RawProfile)
	require.Len(t, out.Sample, 1)
	require.Equal(t, []string{"prod"}, out.Sample[0].Label["environment"])
	_, hasEnv := out.Sample[0].Label["env"]
	require.False(t, hasEnv)

	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.profilesOutgoing))
	require.Equal(t, float64(0), testutil.ToFloat64(c.metrics.profilesDropped))
	require.Equal(t, float64(2), testutil.ToFloat64(c.metrics.samplesProcessed))
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.samplesOutgoing))
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.samplesDropped))
}

func TestAppendDropsProfileWhenNoSamplesRemain(t *testing.T) {
	app := newCaptureAppender()
	c := newComponent(t, app, []*alloy_relabel.Config{
		{
			Action: "labeldrop",
			Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("^env$")},
		},
	})

	in := buildProfile(t, map[string][]string{"env": {"prod"}})
	err := c.Append(t.Context(), labels.FromStrings("service_name", "svc"), []*pyroscope.RawSample{{ID: "1", RawProfile: in}})
	require.NoError(t, err)

	require.Empty(t, app.RawSamples())
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.profilesDropped))
	require.Equal(t, float64(0), testutil.ToFloat64(c.metrics.profilesOutgoing))
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.samplesDropped))
}

func TestAppendPassesThroughNonPPROF(t *testing.T) {
	app := newCaptureAppender()
	c := newComponent(t, app, []*alloy_relabel.Config{
		{
			SourceLabels: []string{"env"},
			TargetLabel:  "environment",
			Action:       "replace",
			Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
			Replacement:  "$1",
		},
	})

	in := []byte("this-is-not-pprof")
	err := c.Append(t.Context(), labels.FromStrings("service_name", "svc"), []*pyroscope.RawSample{{ID: "1", RawProfile: in}})
	require.NoError(t, err)

	rawSamples := app.RawSamples()
	require.Len(t, rawSamples, 1)
	require.Equal(t, in, rawSamples[0].RawProfile)
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.pprofParseFailures))
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.profilesOutgoing))
}

func TestAppendIngestRelabelsProfile(t *testing.T) {
	app := newCaptureAppender()
	c := newComponent(t, app, []*alloy_relabel.Config{
		{
			SourceLabels: []string{"region"},
			TargetLabel:  "zone",
			Action:       "replace",
			Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
			Replacement:  "$1",
		},
		{
			Action: "labeldrop",
			Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("^region$")},
		},
	})

	in := buildProfile(t, map[string][]string{"region": {"us-east-1"}})
	err := c.AppendIngest(t.Context(), &pyroscope.IncomingProfile{
		RawBody: in,
		Labels:  labels.FromStrings("service_name", "svc"),
	})
	require.NoError(t, err)

	profiles := app.IngestProfiles()
	require.Len(t, profiles, 1)
	out := parseProfile(t, profiles[0].RawBody)
	require.Len(t, out.Sample, 1)
	require.Equal(t, []string{"us-east-1"}, out.Sample[0].Label["zone"])
	_, hasRegion := out.Sample[0].Label["region"]
	require.False(t, hasRegion)
}

func TestCacheMetrics(t *testing.T) {
	app := newCaptureAppender()
	c := newComponent(t, app, []*alloy_relabel.Config{
		{
			SourceLabels: []string{"env"},
			TargetLabel:  "environment",
			Action:       "replace",
			Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
			Replacement:  "$1",
		},
	})

	in := buildProfile(t,
		map[string][]string{"env": {"prod"}},
		map[string][]string{"env": {"prod"}},
	)

	err := c.Append(t.Context(), labels.FromStrings("service_name", "svc"), []*pyroscope.RawSample{{ID: "1", RawProfile: in}})
	require.NoError(t, err)

	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.cacheMisses))
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.cacheHits))
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.cacheSize))
}

func newComponent(t *testing.T, app pyroscope.Appendable, rules []*alloy_relabel.Config) *Component {
	t.Helper()

	c, err := New(component.Options{
		Logger:        util.TestLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(component.Exports) {},
	}, Arguments{
		ForwardTo:      []pyroscope.Appendable{app},
		RelabelConfigs: rules,
		MaxCacheSize:   10,
	})
	require.NoError(t, err)
	return c
}

func buildProfile(t testing.TB, sampleLabels ...map[string][]string) []byte {
	t.Helper()

	samples := make([]*profile.Sample, 0, len(sampleLabels))
	for _, lbls := range sampleLabels {
		copied := make(map[string][]string, len(lbls))
		for k, v := range lbls {
			copied[k] = append([]string(nil), v...)
		}
		samples = append(samples, &profile.Sample{Value: []int64{1}, Label: copied})
	}

	p := &profile.Profile{
		SampleType:    []*profile.ValueType{{Type: "samples", Unit: "count"}},
		Sample:        samples,
		TimeNanos:     1,
		DurationNanos: 1,
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        1,
	}

	var buf bytes.Buffer
	require.NoError(t, p.Write(&buf))
	return buf.Bytes()
}

func parseProfile(t testing.TB, data []byte) *profile.Profile {
	t.Helper()
	p, err := profile.ParseData(data)
	require.NoError(t, err)
	return p
}

type captureAppender struct {
	mu             sync.Mutex
	rawSamples     []*pyroscope.RawSample
	ingestProfiles []*pyroscope.IncomingProfile
}

func newCaptureAppender() *captureAppender {
	return &captureAppender{}
}

func (c *captureAppender) Appender() pyroscope.Appender {
	return c
}

func (c *captureAppender) Upload(debuginfo.UploadJob) {}

func (c *captureAppender) Client() debuginfogrpc.DebuginfoServiceClient {
	return nil
}

func (c *captureAppender) Append(_ context.Context, _ labels.Labels, samples []*pyroscope.RawSample) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sample := range samples {
		c.rawSamples = append(c.rawSamples, &pyroscope.RawSample{
			ID:         sample.ID,
			RawProfile: append([]byte(nil), sample.RawProfile...),
		})
	}
	return nil
}

func (c *captureAppender) AppendIngest(_ context.Context, profile *pyroscope.IncomingProfile) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ingestProfiles = append(c.ingestProfiles, &pyroscope.IncomingProfile{
		RawBody:     append([]byte(nil), profile.RawBody...),
		ContentType: append([]string(nil), profile.ContentType...),
		URL:         profile.URL,
		Labels:      profile.Labels.Copy(),
	})
	return nil
}

func (c *captureAppender) RawSamples() []*pyroscope.RawSample {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*pyroscope.RawSample(nil), c.rawSamples...)
}

func (c *captureAppender) IngestProfiles() []*pyroscope.IncomingProfile {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*pyroscope.IncomingProfile(nil), c.ingestProfiles...)
}
