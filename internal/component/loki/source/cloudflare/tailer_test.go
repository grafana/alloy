package cloudflare

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/cloudflare-go"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
)

func TestTailer(t *testing.T) {
	var (
		logger = log.NewNopLogger()
		cfg    = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    3,
			Backoff:    defaultBackoff,
		}
		end      = time.Unix(0, time.Hour.Nanoseconds())
		start    = time.Unix(0, time.Hour.Nanoseconds()-int64(cfg.PullRange))
		handler  = loki.NewCollectingHandler()
		cfClient = newFakeCloudflareClient()
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	// set our end time to be the last time we have a position
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())
	require.NoError(t, err)

	// setup response for the first pull batch of 1 minutes.
	cfClient.On("LogpullReceived", mock.Anything, start, start.Add(time.Duration(cfg.PullRange/3))).Return(&fakeLogIterator{
		logs: []string{
			`{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`,
		},
	}, nil)
	cfClient.On("LogpullReceived", mock.Anything, start.Add(time.Duration(cfg.PullRange/3)), start.Add(time.Duration(2*cfg.PullRange/3))).Return(&fakeLogIterator{
		logs: []string{
			`{"EdgeStartTimestamp":2, "EdgeRequestHost":"bar.com"}`,
		},
	}, nil)
	cfClient.On("LogpullReceived", mock.Anything, start.Add(time.Duration(2*cfg.PullRange/3)), end).Return(&fakeLogIterator{
		logs: []string{
			`{"EdgeStartTimestamp":3, "EdgeRequestHost":"buzz.com"}`,
			`{"EdgeRequestHost":"fuzz.com"}`,
		},
	}, nil)
	// setup empty response for the rest.
	cfClient.On("LogpullReceived", mock.Anything, mock.Anything, mock.Anything).Return(&fakeLogIterator{
		logs: []string{},
	}, nil)
	// replace the client.
	getClient = func(apiKey, zoneID string, fields []string) (Client, error) {
		return cfClient, nil
	}

	ta, err := newTailer(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)
	require.True(t, ta.ready())

	require.Eventually(t, func() bool {
		return len(handler.Received()) == 4
	}, 5*time.Second, 100*time.Millisecond)

	received := handler.Received()
	sort.Slice(received, func(i, j int) bool {
		return received[i].Timestamp.After(received[j].Timestamp)
	})
	for _, e := range received {
		require.Equal(t, model.LabelValue("cloudflare"), e.Labels["job"])
	}
	require.WithinDuration(t, time.Now(), received[0].Timestamp, time.Minute) // no timestamp default to now.
	require.Equal(t, `{"EdgeRequestHost":"fuzz.com"}`, received[0].Line)

	require.Equal(t, `{"EdgeStartTimestamp":3, "EdgeRequestHost":"buzz.com"}`, received[1].Line)
	require.Equal(t, time.Unix(0, 3), received[1].Timestamp)
	require.Equal(t, `{"EdgeStartTimestamp":2, "EdgeRequestHost":"bar.com"}`, received[2].Line)
	require.Equal(t, time.Unix(0, 2), received[2].Timestamp)
	require.Equal(t, `{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`, received[3].Line)
	require.Equal(t, time.Unix(0, 1), received[3].Timestamp)
	cfClient.AssertExpectations(t)
	ta.stop()
	ps.Stop()
	// Make sure we save the last position.
	newPos, _ := ps.Get(positions.CursorKey(cfg.ZoneID), cfg.Labels.String())
	require.Greater(t, newPos, end.UnixNano())
}

func TestTailer_RetryErrorLogpullReceived(t *testing.T) {
	var (
		logger   = log.NewNopLogger()
		end      = time.Unix(0, time.Hour.Nanoseconds())
		start    = time.Unix(0, end.Add(-30*time.Minute).UnixNano())
		handler  = loki.NewCollectingHandler()
		cfClient = newFakeCloudflareClient()
	)
	cfClient.On("LogpullReceived", mock.Anything, start, end).Return(&fakeLogIterator{
		err: ErrorLogpullReceived,
	}, nil).Times(2) // just retry once
	// replace the client
	getClient = func(apiKey, zoneID string, fields []string) (Client, error) {
		return cfClient, nil
	}
	ta := &tailer{
		logger:  logger,
		handler: handler.Receiver(),
		client:  cfClient,
		config: &tailerConfig{
			Labels: make(model.LabelSet),
			Backoff: backoff.Config{
				MinBackoff: 0,
				MaxBackoff: 0,
				MaxRetries: 5,
			},
		},
		metrics: newMetrics(nil),
	}

	require.NoError(t, ta.pull(t.Context(), start, end))
}

func TestTailer_RetryErrorIterating(t *testing.T) {
	var (
		logger   = log.NewNopLogger()
		end      = time.Unix(0, time.Hour.Nanoseconds())
		start    = time.Unix(0, end.Add(-30*time.Minute).UnixNano())
		handler  = loki.NewCollectingHandler()
		cfClient = newFakeCloudflareClient()
	)
	cfClient.On("LogpullReceived", mock.Anything, start, end).Return(&fakeLogIterator{
		logs: []string{
			`{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`,
			`error`,
		},
	}, nil).Once()
	// setup response for the first pull batch of 1 minutes.
	cfClient.On("LogpullReceived", mock.Anything, start, end).Return(&fakeLogIterator{
		logs: []string{
			`{"EdgeStartTimestamp":1, "EdgeRequestHost":"foo.com"}`,
			`{"EdgeStartTimestamp":2, "EdgeRequestHost":"foo.com"}`,
			`{"EdgeStartTimestamp":3, "EdgeRequestHost":"foo.com"}`,
		},
	}, nil).Once()
	cfClient.On("LogpullReceived", mock.Anything, start, end).Return(&fakeLogIterator{
		err: ErrorLogpullReceived,
	}, nil).Once()
	// replace the client.
	getClient = func(apiKey, zoneID string, fields []string) (Client, error) {
		return cfClient, nil
	}
	// retries as fast as possible.
	defaultBackoff.MinBackoff = 0
	defaultBackoff.MaxBackoff = 0
	metrics := newMetrics(prometheus.NewRegistry())
	ta := &tailer{
		logger:  logger,
		handler: handler.Receiver(),
		client:  cfClient,
		config: &tailerConfig{
			Labels:  make(model.LabelSet),
			Backoff: defaultBackoff,
		},
		metrics: metrics,
	}

	require.NoError(t, ta.pull(t.Context(), start, end))
	require.Eventually(t, func() bool {
		return len(handler.Received()) == 4
	}, 5*time.Second, 100*time.Millisecond)
}

func TestTailer_CloudflareTargetError(t *testing.T) {
	var (
		logger = log.NewNopLogger()
		cfg    = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    3,
			Backoff:    defaultBackoff,
		}
		end      = time.Unix(0, time.Hour.Nanoseconds())
		handler  = loki.NewCollectingHandler()
		cfClient = newFakeCloudflareClient()
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	// retries as fast as possible.
	defaultBackoff.MinBackoff = 0
	defaultBackoff.MaxBackoff = 0

	// set our end time to be the last time we have a position
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())
	require.NoError(t, err)

	// setup errors for all retries
	cfClient.On("LogpullReceived", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("no logs"))
	// replace the client.
	getClient = func(apiKey, zoneID string, fields []string) (Client, error) {
		return cfClient, nil
	}

	ta, err := newTailer(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)
	require.True(t, ta.ready())

	// wait for the target to be stopped.
	require.Eventually(t, func() bool {
		return !ta.ready()
	}, 5*time.Second, 100*time.Millisecond)

	require.Len(t, handler.Received(), 0)
	require.GreaterOrEqual(t, cfClient.CallCount(), 5)
	require.NotEmpty(t, ta.details()["error"])
	ta.stop()
	ps.Stop()

	// Make sure we save the last position.
	newEnd, _ := ps.Get(positions.CursorKey(cfg.ZoneID), cfg.Labels.String())
	require.Equal(t, newEnd, end.UnixNano())
}

func TestTailer_CloudflareTargetError168h(t *testing.T) {
	var (
		logger = log.NewNopLogger()
		cfg    = &tailerConfig{
			APIToken:   "foo",
			ZoneID:     "bar",
			Labels:     model.LabelSet{"job": "cloudflare"},
			PullRange:  model.Duration(time.Minute),
			FieldsType: FieldsTypeDefault,
			Workers:    3,
			Backoff:    defaultBackoff,
		}
		end      = time.Unix(0, time.Hour.Nanoseconds())
		handler  = loki.NewCollectingHandler()
		cfClient = newFakeCloudflareClient()
	)
	ps, err := positions.New(logger, positions.Config{
		SyncPeriod:    10 * time.Second,
		PositionsFile: t.TempDir() + "/positions.yml",
	})
	// retries as fast as possible.
	defaultBackoff.MinBackoff = 0
	defaultBackoff.MaxBackoff = 0

	// set our end time to be the last time we have a position
	ps.Put(positions.CursorKey(cfg.ZoneID), cfg.Labels.String(), end.UnixNano())
	require.NoError(t, err)

	// setup errors for all retries
	cfClient.On("LogpullReceived", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("HTTP status 400: bad query: error parsing time: invalid time range: too early: logs older than 168h0m0s are not available"))
	// replace the client.
	getClient = func(_, _ string, _ []string) (Client, error) {
		return cfClient, nil
	}

	ta, err := newTailer(newMetrics(prometheus.NewRegistry()), logger, handler.Receiver(), ps, cfg)
	require.NoError(t, err)
	require.True(t, ta.ready())

	// wait for the target to be stopped.
	require.Eventually(t, func() bool {
		return cfClient.CallCount() >= 5
	}, 5*time.Second, 100*time.Millisecond)

	require.Len(t, handler.Received(), 0)
	require.GreaterOrEqual(t, cfClient.CallCount(), 5)
	ta.stop()
	ps.Stop()

	// Make sure we move on from the save the last position.
	newEnd, _ := ps.Get(positions.CursorKey(cfg.ZoneID), cfg.Labels.String())
	require.Greater(t, newEnd, end.UnixNano())
}

func TestTailer_SplitRequests(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  []pullRequest
	}{
		{
			"perfectly divisible",
			time.Unix(0, 0),
			time.Unix(0, int64(time.Minute)),
			[]pullRequest{
				{start: time.Unix(0, 0), end: time.Unix(0, int64(time.Minute/3))},
				{start: time.Unix(0, int64(time.Minute/3)), end: time.Unix(0, int64(time.Minute*2/3))},
				{start: time.Unix(0, int64(time.Minute*2/3)), end: time.Unix(0, int64(time.Minute))},
			},
		},
		{
			"not divisible",
			time.Unix(0, 0),
			time.Unix(0, int64(time.Minute+1)),
			[]pullRequest{
				{start: time.Unix(0, 0), end: time.Unix(0, int64(time.Minute/3))},
				{start: time.Unix(0, int64(time.Minute/3)), end: time.Unix(0, int64(time.Minute*2/3))},
				{start: time.Unix(0, int64(time.Minute*2/3)), end: time.Unix(0, int64(time.Minute+1))},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRequests(tt.start, tt.end, 3)
			if !assert.Equal(t, tt.want, got) {
				for i := range got {
					if !assert.Equal(t, tt.want[i].start, got[i].start) {
						t.Logf("expected i:%d start: %d , got: %d", i, tt.want[i].start.UnixNano(), got[i].start.UnixNano())
					}
					if !assert.Equal(t, tt.want[i].end, got[i].end) {
						t.Logf("expected i:%d end: %d , got: %d", i, tt.want[i].end.UnixNano(), got[i].end.UnixNano())
					}
				}
			}
		})
	}
}

var ErrorLogpullReceived = errors.New("error logpull received")

type fakeCloudflareClient struct {
	mut sync.RWMutex
	mock.Mock
}

func (f *fakeCloudflareClient) CallCount() int {
	var actualCalls int
	f.mut.RLock()
	for _, call := range f.Calls {
		if call.Method == "LogpullReceived" {
			actualCalls++
		}
	}
	f.mut.RUnlock()
	return actualCalls
}

type fakeLogIterator struct {
	logs    []string
	current string

	err error
}

func (f *fakeLogIterator) Next() bool {
	if len(f.logs) == 0 {
		return false
	}
	f.current = f.logs[0]
	if f.current == `error` {
		f.err = errors.New("error")
		return false
	}
	f.logs = f.logs[1:]
	return true
}
func (f *fakeLogIterator) Err() error                         { return f.err }
func (f *fakeLogIterator) Line() []byte                       { return []byte(f.current) }
func (f *fakeLogIterator) Fields() (map[string]string, error) { return nil, nil }
func (f *fakeLogIterator) Close() error {
	if f.err == ErrorLogpullReceived {
		f.err = nil
	}
	return nil
}

func newFakeCloudflareClient() *fakeCloudflareClient {
	return &fakeCloudflareClient{}
}

func (f *fakeCloudflareClient) LogpullReceived(ctx context.Context, start, end time.Time) (cloudflare.LogpullReceivedIterator, error) {
	f.mut.Lock()
	defer f.mut.Unlock()

	r := f.Called(ctx, start, end)
	if r.Get(0) != nil {
		it := r.Get(0).(cloudflare.LogpullReceivedIterator)
		if it.Err() == ErrorLogpullReceived {
			return it, it.Err()
		}
		return it, nil
	}
	return nil, r.Error(1)
}
