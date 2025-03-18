package write

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

type PushFunc func(context.Context, *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error)

func (p PushFunc) Push(ctx context.Context, r *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error) {
	return p(ctx, r)
}

func Test_Write_FanOut(t *testing.T) {
	var (
		export      Exports
		argument                       = DefaultArguments()
		pushTotal                      = atomic.NewInt32(0)
		serverCount                    = int32(10)
		servers     []*httptest.Server = make([]*httptest.Server, serverCount)
		endpoints   []*EndpointOptions = make([]*EndpointOptions, 0, serverCount)
	)
	argument.ExternalLabels = map[string]string{"foo": "buzz"}
	handlerFn := func(err error) http.Handler {
		_, handler := pushv1connect.NewPusherServiceHandler(PushFunc(
			func(_ context.Context, req *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error) {
				pushTotal.Inc()
				require.Equal(t, "test", req.Header()["X-Test-Header"][0])
				require.Contains(t, req.Header()["User-Agent"][0], "Alloy/")
				require.Equal(t, []*typesv1.LabelPair{
					{Name: "__name__", Value: "test"},
					{Name: "foo", Value: "buzz"},
					{Name: "job", Value: "foo"},
				}, req.Msg.Series[0].Labels)
				require.Equal(t, []byte("pprofraw"), req.Msg.Series[0].Samples[0].RawProfile)
				return &connect.Response[pushv1.PushResponse]{}, err
			},
		))
		return handler
	}

	for i := int32(0); i < serverCount; i++ {
		if i == 0 {
			servers[i] = httptest.NewServer(handlerFn(errors.New("test")))
		} else {
			servers[i] = httptest.NewServer(handlerFn(nil))
		}
		endpoints = append(endpoints, &EndpointOptions{
			URL:               servers[i].URL,
			MinBackoff:        100 * time.Millisecond,
			MaxBackoff:        200 * time.Millisecond,
			MaxBackoffRetries: 1,
			RemoteTimeout:     GetDefaultEndpointOptions().RemoteTimeout,
			Headers: map[string]string{
				"X-Test-Header": "test",
			},
		})
	}
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()
	createReceiver := func(t *testing.T, arg Arguments) pyroscope.Appendable {
		t.Helper()
		var wg sync.WaitGroup
		wg.Add(1)
		c, err := New(component.Options{
			ID:         "1",
			Logger:     util.TestAlloyLogger(t),
			Registerer: prometheus.NewRegistry(),
			OnStateChange: func(e component.Exports) {
				defer wg.Done()
				export = e.(Exports)
			},
		}, arg)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go c.Run(ctx)
		wg.Wait() // wait for the state change to happen
		require.NotNil(t, export.Receiver)
		return export.Receiver
	}

	t.Run("with_failure", func(t *testing.T) {
		argument.Endpoints = endpoints
		r := createReceiver(t, argument)
		pushTotal.Store(0)
		err := r.Appender().Append(context.Background(), labels.FromMap(map[string]string{
			"__name__": "test",
			"__type__": "type",
			"job":      "foo",
			"foo":      "bar",
		}), []*pyroscope.RawSample{
			{RawProfile: []byte("pprofraw")},
		})
		require.EqualErrorf(t, err, "unknown: test", "expected error to be test")
		require.Equal(t, serverCount, pushTotal.Load())
	})

	t.Run("all_success", func(t *testing.T) {
		argument.Endpoints = endpoints[1:]
		r := createReceiver(t, argument)
		pushTotal.Store(0)
		err := r.Appender().Append(context.Background(), labels.FromMap(map[string]string{
			"__name__": "test",
			"__type__": "type",
			"job":      "foo",
			"foo":      "bar",
		}), []*pyroscope.RawSample{
			{RawProfile: []byte("pprofraw")},
		})
		require.NoError(t, err)
		require.Equal(t, serverCount-1, pushTotal.Load())
	})

	t.Run("with_backoff", func(t *testing.T) {
		argument.Endpoints = endpoints[:1]
		argument.Endpoints[0].MaxBackoffRetries = 3
		r := createReceiver(t, argument)
		pushTotal.Store(0)
		err := r.Appender().Append(context.Background(), labels.FromMap(map[string]string{
			"__name__": "test",
			"__type__": "type",
			"job":      "foo",
			"foo":      "bar",
		}), []*pyroscope.RawSample{
			{RawProfile: []byte("pprofraw")},
		})
		require.Error(t, err)
		require.Equal(t, int32(3), pushTotal.Load())
	})
}

func Test_Write_Update(t *testing.T) {
	var (
		export    Exports
		argument  = DefaultArguments()
		pushTotal = atomic.NewInt32(0)
	)
	var wg sync.WaitGroup
	wg.Add(1)
	c, err := New(component.Options{
		ID:         "1",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {
			defer wg.Done()
			export = e.(Exports)
		},
	}, argument)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	wg.Wait() // wait for the state change to happen
	require.NotNil(t, export.Receiver)
	// First one is a noop
	err = export.Receiver.Appender().Append(context.Background(), labels.FromMap(map[string]string{
		"__name__": "test",
	}), []*pyroscope.RawSample{
		{RawProfile: []byte("pprofraw")},
	})
	require.NoError(t, err)

	_, handler := pushv1connect.NewPusherServiceHandler(PushFunc(
		func(_ context.Context, req *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error) {
			pushTotal.Inc()
			return &connect.Response[pushv1.PushResponse]{}, err
		},
	))
	server := httptest.NewServer(handler)
	defer server.Close()
	argument.Endpoints = []*EndpointOptions{
		{
			URL:           server.URL,
			RemoteTimeout: GetDefaultEndpointOptions().RemoteTimeout,
		},
	}
	wg.Add(1)
	require.NoError(t, c.Update(argument))
	wg.Wait()
	err = export.Receiver.Appender().Append(context.Background(), labels.FromMap(map[string]string{
		"__name__": "test",
	}), []*pyroscope.RawSample{
		{RawProfile: []byte("pprofraw")},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), pushTotal.Load())
}

func Test_Unmarshal_Config(t *testing.T) {
	var arg Arguments
	syntax.Unmarshal([]byte(`
	endpoint {
		url = "http://localhost:4100"
		remote_timeout = "10s"
	}
	endpoint {
		url = "http://localhost:4200"
		remote_timeout = "5s"
		min_backoff_period = "1s"
		max_backoff_period = "10s"
		max_backoff_retries = 10
	}
	external_labels = {
		"foo" = "bar",
	}`), &arg)
	require.Equal(t, "http://localhost:4100", arg.Endpoints[0].URL)
	require.Equal(t, "http://localhost:4200", arg.Endpoints[1].URL)
	require.Equal(t, time.Second*10, arg.Endpoints[0].RemoteTimeout)
	require.Equal(t, time.Second*5, arg.Endpoints[1].RemoteTimeout)
	require.Equal(t, "bar", arg.ExternalLabels["foo"])
	require.Equal(t, time.Second, arg.Endpoints[1].MinBackoff)
	require.Equal(t, time.Second*10, arg.Endpoints[1].MaxBackoff)
	require.Equal(t, 10, arg.Endpoints[1].MaxBackoffRetries)
}

func TestBadAlloyConfig(t *testing.T) {
	exampleAlloyConfig := `
	endpoint {
		url = "http://localhost:4100"
		remote_timeout = "10s"
		bearer_token = "token"
		bearer_token_file = "/path/to/file.token"
	}
	external_labels = {
		"foo" = "bar",
	}
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

func Test_Write_AppendIngest(t *testing.T) {
	var (
		export      Exports
		argument    = DefaultArguments()
		appendCount = atomic.NewInt32(0)
		serverCount = int32(3)
		servers     = make([]*httptest.Server, serverCount)
		endpoints   = make([]*EndpointOptions, 0, serverCount)
	)

	testData := []byte("test-profile-data")
	argument.ExternalLabels = map[string]string{
		"env":     "prod",      // Should override env=staging
		"cluster": "cluster-1", // Should be added
	}

	handlerFn := func(expectedPath string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			appendCount.Inc()
			require.Equal(t, expectedPath, r.URL.Path, "Unexpected path")

			// Header assertions
			require.Equal(t, "endpoint-value", r.Header.Get("X-Test-Header"))
			require.Equal(t, []string{"profile-value1", "profile-value2"}, r.Header["X-Profile-Header"])

			// Label assertions - parse the name parameter once
			ls, err := labelset.Parse(r.URL.Query().Get("name"))
			require.NoError(t, err)
			labels := ls.Labels()

			// Check each label individually
			require.Equal(t, "my.awesome.app.cpu", labels["__name__"], "Base name should be preserved")
			require.Equal(t, "prod", labels["env"], "External label should override profile label")
			require.Equal(t, "cluster-1", labels["cluster"], "External label should be added")
			require.Equal(t, "us-west-1", labels["region"], "Profile-only label should be preserved")

			// Check non-label query params
			require.Equal(t, "value", r.URL.Query().Get("key"), "Original query parameter should be preserved")

			// Body assertion
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err, "Failed to read request body")
			require.Equal(t, testData, body, "Unexpected body content")
			w.WriteHeader(http.StatusOK)
		}
	}

	for i := int32(0); i < serverCount; i++ {
		servers[i] = httptest.NewServer(handlerFn("/ingest"))
		endpoints = append(endpoints, &EndpointOptions{
			URL:           servers[i].URL,
			RemoteTimeout: GetDefaultEndpointOptions().RemoteTimeout,
			Headers: map[string]string{
				"X-Test-Header": "endpoint-value",
			},
		})
	}
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	argument.Endpoints = endpoints

	// Create the receiver
	var wg sync.WaitGroup
	wg.Add(1)
	c, err := New(component.Options{
		ID:         "test-write",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {
			defer wg.Done()
			export = e.(Exports)
		},
	}, argument)
	require.NoError(t, err, "Failed to create component")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	wg.Wait() // wait for the state change to happen
	require.NotNil(t, export.Receiver, "Receiver is nil")

	incomingProfile := &pyroscope.IncomingProfile{
		RawBody: testData,
		Headers: http.Header{
			"X-Test-Header":    []string{"profile-value"},                    // This should be overridden by endpoint
			"X-Profile-Header": []string{"profile-value1", "profile-value2"}, // This should be preserved
		},
		URL: &url.URL{
			Path:     "/ingest",
			RawQuery: "key=value",
		},
		Labels: labels.FromMap(map[string]string{
			"__name__": "my.awesome.app.cpu",
			"env":      "staging",
			"region":   "us-west-1",
		}),
	}

	err = export.Receiver.Appender().AppendIngest(context.Background(), incomingProfile)
	require.NoError(t, err)
	require.Equal(t, serverCount, appendCount.Load())
}

func TestAppendIngestLabelTransformation(t *testing.T) {
	var (
		export      Exports
		appendCount = atomic.NewInt32(0)
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appendCount.Inc()

		// Parse labels from query
		ls, err := labelset.Parse(r.URL.Query().Get("name"))
		require.NoError(t, err)
		labels := ls.Labels()

		// Verify __name__ matches service_name after transformation
		require.Equal(t, "my-service-grafana", labels["__name__"])
		require.Equal(t, "my-service-grafana", labels["service_name"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create component with a relabel rule that modifies service_name
	argument := DefaultArguments()
	argument.Endpoints = []*EndpointOptions{{
		URL:           server.URL,
		RemoteTimeout: GetDefaultEndpointOptions().RemoteTimeout,
	}}

	var wg sync.WaitGroup
	wg.Add(1)
	c, err := New(component.Options{
		ID:         "test-write",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {
			defer wg.Done()
			export = e.(Exports)
		},
	}, argument)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	wg.Wait()
	require.NotNil(t, export.Receiver)

	// Send profile
	incomingProfile := &pyroscope.IncomingProfile{
		Labels: labels.FromMap(map[string]string{
			"__name__":     "original-name",
			"service_name": "my-service-grafana",
		}),
		URL: &url.URL{Path: "/ingest"},
	}

	err = export.Receiver.Appender().AppendIngest(context.Background(), incomingProfile)
	require.NoError(t, err)
	require.Equal(t, int32(1), appendCount.Load())
}

func Test_Write_AppendIngest_InvalidLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	argument := DefaultArguments()
	argument.Endpoints = []*EndpointOptions{{
		URL:           server.URL,
		RemoteTimeout: GetDefaultEndpointOptions().RemoteTimeout,
	}}

	var wg sync.WaitGroup
	var export Exports
	wg.Add(1)
	c, err := New(component.Options{
		ID:         "test-write-invalid",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {
			defer wg.Done()
			export = e.(Exports)
		},
	}, argument)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	wg.Wait()
	require.NotNil(t, export.Receiver)

	testCases := []struct {
		name    string
		labels  labels.Labels
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid labels",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"valid_label", "value",
				"__name__", "test-name",
			),
			wantErr: false,
		},
		{
			name: "duplicate labels",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"duplicate", "value1",
				"duplicate", "value2",
			),
			wantErr: true,
			errMsg:  "duplicate label name",
		},
		{
			name: "invalid label name",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"invalid-label", "value",
			),
			wantErr: true,
			errMsg:  labelset.ErrInvalidLabelName.Error(),
		},
		{
			name: "invalid label value",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"valid_label", string([]byte{0xff}), // Invalid UTF-8
			),
			wantErr: true,
			errMsg:  "invalid label value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			incomingProfile := &pyroscope.IncomingProfile{
				RawBody: []byte("test-data"),
				Labels:  tc.labels,
				URL:     &url.URL{Path: "/ingest"},
			}

			err = export.Receiver.Appender().AppendIngest(context.Background(), incomingProfile)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_Write_FanOut_ValidateLabels(t *testing.T) {
	_, handler := pushv1connect.NewPusherServiceHandler(PushFunc(
		func(_ context.Context, req *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error) {
			return &connect.Response[pushv1.PushResponse]{}, nil
		},
	))

	server := httptest.NewServer(handler)
	defer server.Close()

	argument := DefaultArguments()
	argument.Endpoints = []*EndpointOptions{{
		URL:           server.URL,
		RemoteTimeout: GetDefaultEndpointOptions().RemoteTimeout,
	}}

	var wg sync.WaitGroup
	var export Exports
	wg.Add(1)
	c, err := New(component.Options{
		ID:         "test-write-fanout-validate-labels",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {
			defer wg.Done()
			export = e.(Exports)
		},
	}, argument)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	wg.Wait()
	require.NotNil(t, export.Receiver)

	testCases := []struct {
		name    string
		labels  labels.Labels
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid labels",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"valid_label", "value",
				"__name__", "test-name",
			),
			wantErr: false,
		},
		{
			name: "duplicate labels",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"duplicate", "value1",
				"duplicate", "value2",
			),
			wantErr: true,
			errMsg:  "duplicate label name",
		},
		{
			name: "invalid label name",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"invalid-label-name", "value",
			),
			wantErr: true,
			errMsg:  labelset.ErrInvalidLabelName.Error(),
		},
		{
			name: "duplicate reserved labels",
			labels: labels.FromStrings(
				"__name__", "test-service",
				"__name__", "test-service-2",
			),
			wantErr: true,
			errMsg:  "duplicate label name",
		},
		{
			name: "invalid label value",
			labels: labels.FromStrings(
				"service_name", "test-service",
				"valid_label", string([]byte{0xff}), // Invalid UTF-8
			),
			wantErr: true,
			errMsg:  "invalid label value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err = export.Receiver.Appender().Append(context.Background(), tc.labels, []*pyroscope.RawSample{
				{RawProfile: []byte("test-data")},
			})

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid labels in profile")
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
