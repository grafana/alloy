package pyroscope

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/url"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	alloypyroscope "github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

func TestComponentConvertsAndForwardsPprof(t *testing.T) {
	ctx := componenttest.TestContext(t)
	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.receiver.pyroscope")
	require.NoError(t, err)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		output {
			// no-op: overridden by the test.
		}
	`), &args))

	profilesCh := make(chan pprofile.Profiles, 1)
	args.Output = profilesOutput(func(_ context.Context, profiles pprofile.Profiles) error {
		profilesCh <- profiles
		return nil
	})

	go func() {
		require.NoError(t, ctrl.Run(ctx, args))
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(Exports)
	err = exports.Receiver.Appender().Append(t.Context(), labels.FromStrings(
		alloypyroscope.LabelName, "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
		alloypyroscope.LabelNameDelta, "false",
		alloypyroscope.LabelServiceName, "checkout-api",
		alloypyroscope.LabelOtelScopeName, alloypyroscope.ScopeNameEBPF,
		alloypyroscope.LabelOtelScopeVersion, "v1.2.3",
		"environment", "production",
		"__meta_kubernetes_pod_name", "checkout-123",
	), []*alloypyroscope.RawSample{
		{ID: "profile-1", RawProfile: testPprof(t, 42)},
	})
	require.NoError(t, err)

	select {
	case profiles := <-profilesCh:
		assertConvertedProfiles(t, profiles)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for converted profiles")
	}
}

func TestProfilesReceiverAppendMergesSamples(t *testing.T) {
	var consumed pprofile.Profiles
	receiver := newTestReceiver(func(_ context.Context, profiles pprofile.Profiles) error {
		consumed = profiles
		return nil
	})

	err := receiver.Append(t.Context(), labels.FromStrings(
		alloypyroscope.LabelServiceName, "checkout-api",
	), []*alloypyroscope.RawSample{
		{RawProfile: testPprof(t, 10)},
		{RawProfile: testPprof(t, 20)},
	})
	require.NoError(t, err)
	require.Equal(t, 2, consumed.ResourceProfiles().Len())
	require.Equal(t, 2, consumed.ProfileCount())
	require.Equal(t, 2, consumed.SampleCount())
}

func TestProfilesReceiverAppendIngestConvertsPprof(t *testing.T) {
	tests := []struct {
		name        string
		contentType []string
	}{
		{name: "application/octet-stream", contentType: []string{"application/octet-stream"}},
		{name: "missing content type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var consumed pprofile.Profiles
			receiver := newTestReceiver(func(_ context.Context, profiles pprofile.Profiles) error {
				consumed = profiles
				return nil
			})

			err := receiver.AppendIngest(t.Context(), &alloypyroscope.IncomingProfile{
				RawBody:     testPprof(t, 42),
				ContentType: tt.contentType,
				URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
				Labels: labels.FromStrings(
					alloypyroscope.LabelServiceName, "checkout-api",
				),
			})
			require.NoError(t, err)
			require.Equal(t, 1, consumed.ProfileCount())
			require.Equal(t, 1, consumed.SampleCount())
		})
	}
}

func TestProfilesReceiverAppendIngestRejectsUnsupportedFormats(t *testing.T) {
	pprofMultipartBody, pprofMultipartContentType := testMultipartBody(t, "profile", testPprof(t, 42))
	jfrMultipartBody, jfrMultipartContentType := testMultipartBody(t, "jfr", []byte("jfr data"))

	tests := []struct {
		name     string
		incoming *alloypyroscope.IncomingProfile
	}{
		{
			name: "missing URL",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     testPprof(t, 42),
				ContentType: []string{"application/octet-stream"},
			},
		},
		{
			name: "missing format",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     testPprof(t, 42),
				ContentType: []string{"application/octet-stream"},
				URL:         &url.URL{Path: "/ingest"},
			},
		},
		{
			name: "JFR",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     []byte("jfr data"),
				ContentType: []string{"application/octet-stream"},
				URL:         &url.URL{Path: "/ingest", RawQuery: "format=jfr"},
			},
		},
		{
			name: "speedscope",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     []byte(`{"$schema":"https://www.speedscope.app/file-format-schema.json"}`),
				ContentType: []string{"application/json"},
				URL:         &url.URL{Path: "/ingest", RawQuery: "format=speedscope"},
			},
		},
		{
			name: "multipart pprof without format",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     pprofMultipartBody,
				ContentType: []string{pprofMultipartContentType},
				URL:         &url.URL{Path: "/ingest"},
			},
		},
		{
			name: "multipart body declared as pprof",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     pprofMultipartBody,
				ContentType: []string{pprofMultipartContentType},
				URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
			},
		},
		{
			name: "multipart JFR",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     jfrMultipartBody,
				ContentType: []string{jfrMultipartContentType},
				URL:         &url.URL{Path: "/ingest", RawQuery: "format=jfr"},
			},
		},
		{
			name: "malformed content type",
			incoming: &alloypyroscope.IncomingProfile{
				RawBody:     testPprof(t, 42),
				ContentType: []string{"multipart/form-data; boundary="},
				URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumeCalls := 0
			receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
				consumeCalls++
				return nil
			})

			err := receiver.AppendIngest(t.Context(), tt.incoming)
			require.ErrorIs(t, err, ErrUnsupportedIngestFormat)
			require.Zero(t, consumeCalls)
		})
	}
}

func TestProfilesReceiverAppendIngestRejectsMalformedPprof(t *testing.T) {
	consumeCalls := 0
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		consumeCalls++
		return nil
	})

	err := receiver.AppendIngest(t.Context(), &alloypyroscope.IncomingProfile{
		RawBody:     []byte("not a pprof profile"),
		ContentType: []string{"application/octet-stream"},
		URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
	})
	require.ErrorContains(t, err, "convert ingested pprof: parse pprof profile")
	require.NotErrorIs(t, err, ErrUnsupportedIngestFormat)
	require.Zero(t, consumeCalls)
}

func TestProfilesReceiverAppendIngestHonorsCancelledContext(t *testing.T) {
	consumeCalls := 0
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		consumeCalls++
		return nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := receiver.AppendIngest(ctx, &alloypyroscope.IncomingProfile{
		RawBody:     testPprof(t, 42),
		ContentType: []string{"application/octet-stream"},
		URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, consumeCalls)
}

func TestProfilesReceiverAppendIngestHonorsCancellationAfterConversion(t *testing.T) {
	consumeCalls := 0
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		consumeCalls++
		return nil
	})
	ctx := newCancelOnSecondCheckContext(t.Context())

	err := receiver.AppendIngest(ctx, &alloypyroscope.IncomingProfile{
		RawBody:     testPprof(t, 42),
		ContentType: []string{"application/octet-stream"},
		URL:         &url.URL{Path: "/ingest", RawQuery: "format=pprof"},
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, consumeCalls)
}

func TestProfilesReceiverAppendHonorsCancelledContext(t *testing.T) {
	consumeCalls := 0
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		consumeCalls++
		return nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := receiver.Append(ctx, labels.EmptyLabels(), []*alloypyroscope.RawSample{
		{RawProfile: testPprof(t, 42)},
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, consumeCalls)
}

func TestProfilesReceiverRejectsInvalidProfilesBeforeConsuming(t *testing.T) {
	consumeCalls := 0
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		consumeCalls++
		return nil
	})

	err := receiver.Append(t.Context(), labels.EmptyLabels(), []*alloypyroscope.RawSample{
		{RawProfile: testPprof(t, 42)},
		{RawProfile: []byte("not a pprof profile")},
	})
	require.ErrorContains(t, err, "convert sample 1: parse pprof profile")
	require.Zero(t, consumeCalls)

	err = receiver.Append(t.Context(), labels.EmptyLabels(), []*alloypyroscope.RawSample{nil})
	require.ErrorContains(t, err, "convert sample 0: sample is nil")
	require.Zero(t, consumeCalls)

	err = receiver.Append(t.Context(), labels.EmptyLabels(), []*alloypyroscope.RawSample{{
		ID:         "profile-id",
		RawProfile: []byte("not a pprof profile"),
	}})
	require.ErrorContains(t, err, `convert sample 0 ("profile-id"): parse pprof profile`)
	require.Zero(t, consumeCalls)

	err = receiver.AppendIngest(t.Context(), nil)
	require.ErrorContains(t, err, "ingested profile is nil")
	require.Zero(t, consumeCalls)
}

func TestProfilesReceiverPropagatesConsumerErrors(t *testing.T) {
	wantErr := errors.New("downstream unavailable")
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		return wantErr
	})

	err := receiver.Append(t.Context(), labels.EmptyLabels(), []*alloypyroscope.RawSample{
		{RawProfile: testPprof(t, 42)},
	})
	require.ErrorIs(t, err, wantErr)
}

func TestProfilesReceiverDoesNotConsumeEmptyBatch(t *testing.T) {
	consumeCalls := 0
	receiver := newTestReceiver(func(_ context.Context, _ pprofile.Profiles) error {
		consumeCalls++
		return nil
	})

	require.NoError(t, receiver.Append(t.Context(), labels.EmptyLabels(), nil))
	require.Zero(t, consumeCalls)
}

func TestComponentRequiresProfilesOutput(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr string
	}{
		{
			name:    "missing output block",
			args:    Arguments{},
			wantErr: "output block must be provided",
		},
		{
			name:    "empty output block",
			args:    Arguments{Output: &otelcol.ConsumerArguments{}},
			wantErr: "output.profiles must contain at least one consumer",
		},
		{
			name: "nil profiles consumer",
			args: Arguments{Output: &otelcol.ConsumerArguments{
				Profiles: []otelcol.Consumer{nil},
			}},
			wantErr: "output.profiles must contain at least one consumer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(component.Options{}, tt.args)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

type cancelOnSecondCheckContext struct {
	context.Context
	checks int
	done   chan struct{}
}

func newCancelOnSecondCheckContext(parent context.Context) *cancelOnSecondCheckContext {
	return &cancelOnSecondCheckContext{
		Context: parent,
		done:    make(chan struct{}),
	}
}

func (c *cancelOnSecondCheckContext) Done() <-chan struct{} {
	return c.done
}

func (c *cancelOnSecondCheckContext) Err() error {
	c.checks++
	if c.checks > 1 {
		if c.checks == 2 {
			close(c.done)
		}
		return context.Canceled
	}
	return nil
}

func newTestReceiver(consume func(context.Context, pprofile.Profiles) error) *profilesReceiver {
	receiver := &profilesReceiver{}
	receiver.setConsumer(&fakeconsumer.Consumer{ConsumeProfilesFunc: consume})
	return receiver
}

func profilesOutput(consume func(context.Context, pprofile.Profiles) error) *otelcol.ConsumerArguments {
	consumer := &fakeconsumer.Consumer{ConsumeProfilesFunc: consume}
	return &otelcol.ConsumerArguments{Profiles: []otelcol.Consumer{consumer}}
}

func testMultipartBody(t *testing.T, field string, body []byte) ([]byte, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(field, field)
	require.NoError(t, err)
	_, err = part.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buf.Bytes(), writer.FormDataContentType()
}

func assertConvertedProfiles(t *testing.T, profiles pprofile.Profiles) {
	t.Helper()

	require.Equal(t, 1, profiles.ResourceProfiles().Len())
	require.Equal(t, 1, profiles.ProfileCount())
	require.Equal(t, 1, profiles.SampleCount())

	resourceProfiles := profiles.ResourceProfiles().At(0)
	attrs := resourceProfiles.Resource().Attributes().AsRaw()
	require.Equal(t, "checkout-api", attrs["service.name"])
	require.Equal(t, "production", attrs["environment"])
	require.Equal(t, "process_cpu:cpu:nanoseconds:cpu:nanoseconds", attrs[alloypyroscope.LabelName])
	require.NotContains(t, attrs, alloypyroscope.LabelServiceName)
	require.NotContains(t, attrs, alloypyroscope.LabelNameDelta)
	require.NotContains(t, attrs, alloypyroscope.LabelOtelScopeName)
	require.NotContains(t, attrs, alloypyroscope.LabelOtelScopeVersion)
	require.NotContains(t, attrs, "__meta_kubernetes_pod_name")

	require.Equal(t, 1, resourceProfiles.ScopeProfiles().Len())
	scopeProfiles := resourceProfiles.ScopeProfiles().At(0)
	require.Equal(t, alloypyroscope.ScopeNameEBPF, scopeProfiles.Scope().Name())
	require.Equal(t, "v1.2.3", scopeProfiles.Scope().Version())
	require.Equal(t, 1, scopeProfiles.Profiles().Len())

	convertedProfile := scopeProfiles.Profiles().At(0)
	dictionary := profiles.Dictionary()
	require.Equal(t, "cpu", dictionary.StringTable().At(int(convertedProfile.SampleType().TypeStrindex())))
	require.Equal(t, "nanoseconds", dictionary.StringTable().At(int(convertedProfile.SampleType().UnitStrindex())))

	convertedSample := convertedProfile.Samples().At(0)
	require.Equal(t, int64(42), convertedSample.Values().At(0))

	var foundEnvironment, foundThreadID bool
	for i := 0; i < convertedSample.AttributeIndices().Len(); i++ {
		attribute := dictionary.AttributeTable().At(int(convertedSample.AttributeIndices().At(i)))
		key := dictionary.StringTable().At(int(attribute.KeyStrindex()))
		switch key {
		case "environment":
			foundEnvironment = true
			require.Equal(t, 1, attribute.Value().Slice().Len())
			require.Equal(t, "sample-production", attribute.Value().Slice().At(0).Str())
		case "thread.id":
			foundThreadID = true
			require.Equal(t, 1, attribute.Value().Slice().Len())
			require.Equal(t, int64(7), attribute.Value().Slice().At(0).Int())
			require.Equal(t, "id", dictionary.StringTable().At(int(attribute.UnitStrindex())))
		}
	}
	require.True(t, foundEnvironment, "embedded pprof string label was not converted to a sample attribute")
	require.True(t, foundThreadID, "embedded pprof numeric label was not converted to a sample attribute")
}

func testPprof(t *testing.T, value int64) []byte {
	t.Helper()

	mapping := &profile.Mapping{
		ID:             1,
		Start:          0x1000,
		Limit:          0x2000,
		File:           "checkout-api",
		BuildID:        "build-id",
		HasFunctions:   true,
		HasFilenames:   true,
		HasLineNumbers: true,
	}
	function := &profile.Function{
		ID:         1,
		Name:       "main.checkout",
		SystemName: "main.checkout",
		Filename:   "checkout.go",
		StartLine:  10,
	}
	location := &profile.Location{
		ID:      1,
		Mapping: mapping,
		Address: 0x1010,
		Line: []profile.Line{{
			Function: function,
			Line:     12,
		}},
	}

	prof := &profile.Profile{
		SampleType: []*profile.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		Sample: []*profile.Sample{{
			Location: []*profile.Location{location},
			Value:    []int64{value},
			Label: map[string][]string{
				"environment": {"sample-production"},
			},
			NumLabel: map[string][]int64{
				"thread.id": {7},
			},
			NumUnit: map[string][]string{
				"thread.id": {"id"},
			},
		}},
		Mapping:       []*profile.Mapping{mapping},
		Location:      []*profile.Location{location},
		Function:      []*profile.Function{function},
		TimeNanos:     time.Unix(1_700_000_000, 0).UnixNano(),
		DurationNanos: int64(10 * time.Second),
		PeriodType:    &profile.ValueType{Type: "cpu", Unit: "nanoseconds"},
		Period:        int64(10 * time.Millisecond),
	}

	var buf bytes.Buffer
	require.NoError(t, prof.Write(&buf))
	return buf.Bytes()
}
