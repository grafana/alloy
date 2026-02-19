package write

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/grafana/alloy/internal/component/pyroscope"
	pyrotestlogger "github.com/grafana/alloy/internal/component/pyroscope/util/testlog"
	"github.com/grafana/alloy/syntax"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/atomic"
)

type PushFunc func(context.Context, *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error)

func (p PushFunc) Push(ctx context.Context, r *connect.Request[pushv1.PushRequest]) (*connect.Response[pushv1.PushResponse], error) {
	return p(ctx, r)
}

func Test_Write_FanOut(t *testing.T) {
	var (
		export      Exports
		argument    = DefaultArguments()
		pushTotal   = atomic.NewInt32(0)
		serverCount = int32(10)
		servers     = make([]*httptest.Server, serverCount)
		endpoints   = make([]*EndpointOptions, 0, serverCount)
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
				require.Equal(t, "test-request-id", req.Msg.Series[0].Samples[0].ID)
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
		c, err := New(
			pyrotestlogger.TestLogger(t),
			noop.Tracer{},
			prometheus.NewRegistry(),
			func(e Exports) {
				defer wg.Done()
				export = e
			},
			"Alloy/239",
			"",
			t.TempDir(),
			arg,
		)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(t.Context())
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
		err := r.Appender().Append(t.Context(), labels.FromMap(map[string]string{
			"__name__": "test",
			"__type__": "type",
			"job":      "foo",
			"foo":      "bar",
		}), []*pyroscope.RawSample{
			{ID: "test-request-id", RawProfile: []byte("pprofraw")},
		})
		require.ErrorContains(t, err, "unknown: test")
		require.Equal(t, serverCount, pushTotal.Load())
	})

	t.Run("all_success", func(t *testing.T) {
		argument.Endpoints = endpoints[1:]
		r := createReceiver(t, argument)
		pushTotal.Store(0)
		err := r.Appender().Append(t.Context(), labels.FromMap(map[string]string{
			"__name__": "test",
			"__type__": "type",
			"job":      "foo",
			"foo":      "bar",
		}), []*pyroscope.RawSample{
			{ID: "test-request-id", RawProfile: []byte("pprofraw")},
		})
		require.NoError(t, err)
		require.Equal(t, serverCount-1, pushTotal.Load())
	})

	t.Run("with_backoff", func(t *testing.T) {
		argument.Endpoints = endpoints[:1]
		argument.Endpoints[0].MaxBackoffRetries = 3
		r := createReceiver(t, argument)
		pushTotal.Store(0)
		err := r.Appender().Append(t.Context(), labels.FromMap(map[string]string{
			"__name__": "test",
			"__type__": "type",
			"job":      "foo",
			"foo":      "bar",
		}), []*pyroscope.RawSample{
			{ID: "test-request-id", RawProfile: []byte("pprofraw")},
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
	c, err := New(
		pyrotestlogger.TestLogger(t),
		noop.Tracer{},
		prometheus.NewRegistry(),
		func(e Exports) {
			defer wg.Done()
			export = e
		},
		"Alloy/239",
		"",
		t.TempDir(),
		argument,
	)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go c.Run(ctx)
	wg.Wait() // wait for the state change to happen
	require.NotNil(t, export.Receiver)
	// First one is a noop
	err = export.Receiver.Appender().Append(t.Context(), labels.FromMap(map[string]string{
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
	err = export.Receiver.Appender().Append(t.Context(), labels.FromMap(map[string]string{
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

type AppendIngestTestSuite struct {
	suite.Suite
	servers      []*httptest.Server
	component    *Component
	export       Exports
	requestCount *atomic.Int32
	ctx          context.Context
	cancel       context.CancelFunc
	testData     []byte
	arguments    Arguments
}

func (s *AppendIngestTestSuite) SetupTest() {
	s.requestCount = new(atomic.Int32)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.testData = []byte("test-profile-data")
	s.servers = nil
}

func (s *AppendIngestTestSuite) TearDownTest() {
	s.cancel()
	for _, server := range s.servers {
		server.Close()
	}
}

func (s *AppendIngestTestSuite) newServer(handlerLogic func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	ss := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requestCount.Inc()
		s.Require().Equal("/ingest", r.URL.Path, "Unexpected path")
		if handlerLogic != nil {
			handlerLogic(w, r)
		}
	}))
	s.servers = append(s.servers, ss)
	return ss
}

func (s *AppendIngestTestSuite) newComponent(argument Arguments) {
	var wg sync.WaitGroup
	wg.Add(1)
	var err error
	s.arguments = argument
	s.component, err = New(
		pyrotestlogger.TestLogger(s.T()),
		noop.Tracer{},
		prometheus.NewRegistry(),
		func(e Exports) {
			defer wg.Done()
			s.export = e
		},
		"Alloy/239",
		"",
		s.T().TempDir(),
		argument,
	)
	s.Require().NoError(err)

	go s.component.Run(s.ctx)
	wg.Wait()
	s.Require().NotNil(s.export.Receiver)
}

func (s *AppendIngestTestSuite) newProfile(labelMap map[string]string) *pyroscope.IncomingProfile {
	profile := &pyroscope.IncomingProfile{
		RawBody: s.testData,
		URL: &url.URL{
			Path:     "/ingest",
			RawQuery: "key=value",
		},
	}
	profile.Labels = labels.FromMap(labelMap)
	profile.ContentType = []string{"ct1", "ct2"}
	return profile
}

func (s *AppendIngestTestSuite) newEndpoint(server *httptest.Server, headers map[string]string) *EndpointOptions {
	return &EndpointOptions{
		URL:               server.URL,
		RemoteTimeout:     10 * time.Second,
		MaxBackoffRetries: 3,
		MinBackoff:        1 * time.Millisecond,
		MaxBackoff:        1 * time.Millisecond,
		Headers:           headers,
	}
}

func (s *AppendIngestTestSuite) TestBasicFunctionality() {
	server := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		ls, err := labelset.Parse(r.URL.Query().Get("name"))
		s.Require().NoError(err)

		s.Equal("i-am-so-good", r.Header.Get("X-Good-Header"))
		s.Equal([]string{"ct1", "ct2"}, r.Header["Content-Type"])
		s.Equal("my.awesome.app.cpu", ls.Labels()["__name__"])
		s.Equal("prod", ls.Labels()["env"])
		s.Equal("cluster-1", ls.Labels()["cluster"])
		s.Equal("us-west-1", ls.Labels()["region"])
		s.Equal("value", r.URL.Query().Get("key"))

		body, err := io.ReadAll(r.Body)
		s.Require().NoError(err)
		s.Equal(s.testData, body)

		w.WriteHeader(http.StatusOK)
	})

	s.newComponent(Arguments{
		ExternalLabels: map[string]string{
			"env":     "prod",
			"cluster": "cluster-1",
		},
		Endpoints: []*EndpointOptions{s.newEndpoint(server, map[string]string{
			"ContentType":   "evil-content-type",
			"X-Good-Header": "i-am-so-good",
		})},
	})

	profile := s.newProfile(map[string]string{
		"__name__": "my.awesome.app.cpu",
		"env":      "staging",
		"region":   "us-west-1",
	})

	err := s.export.Receiver.Appender().AppendIngest(s.ctx, profile)
	s.NoError(err)
	s.Equal(int32(1), s.requestCount.Load())
}

func (s *AppendIngestTestSuite) TestErrorHandling() {
	const serverCount = 3
	for i := int32(0); i < serverCount; i++ {
		s.newServer(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Server-ID") == "1" {
				err := errors.New("I don't like your profile")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(err.Error()))
			} else {
				w.WriteHeader(http.StatusOK)
			}
		})
	}

	argument := DefaultArguments()
	for i, server := range s.servers {
		argument.Endpoints = append(argument.Endpoints, s.newEndpoint(server, map[string]string{
			"X-Server-ID": strconv.Itoa(i),
		}))
	}

	s.newComponent(argument)

	profile := s.newProfile(map[string]string{
		"__name__": "test.profile",
	})

	err := s.export.Receiver.Appender().AppendIngest(s.ctx, profile)
	s.ErrorContains(err, "remote error: pyroscope write error: status=400 msg=I don't like your profile")
}

func (s *AppendIngestTestSuite) TestRetryLogic() {
	server := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		if s.requestCount.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service temporarily unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	s.newComponent(Arguments{
		Endpoints: []*EndpointOptions{s.newEndpoint(server, nil)},
	})

	profile := s.newProfile(map[string]string{
		"__name__": "test.profile",
		"service":  "test-service",
	})

	err := s.export.Receiver.Appender().AppendIngest(s.ctx, profile)
	s.NoError(err)
	s.Equal(int32(2), s.requestCount.Load())
}

func (s *AppendIngestTestSuite) TestRetryExhaustion() {
	server := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service always unavailable"))
	})

	s.newComponent(Arguments{
		Endpoints: []*EndpointOptions{s.newEndpoint(server, nil)},
	})

	profile := s.newProfile(map[string]string{
		"__name__": "test.profile",
		"service":  "test-service",
	})

	err := s.export.Receiver.Appender().AppendIngest(s.ctx, profile)
	s.Error(err)
	s.Contains(err.Error(), "pyroscope write error: status=503 msg=service always unavailable")
	s.Positive(s.arguments.Endpoints[0].MaxBackoffRetries)
	s.Equal(int32(s.arguments.Endpoints[0].MaxBackoffRetries), s.requestCount.Load())
}

func (s *AppendIngestTestSuite) TestLabelTransformation() {
	server := s.newServer(func(w http.ResponseWriter, r *http.Request) {
		ls, err := labelset.Parse(r.URL.Query().Get("name"))
		s.Require().NoError(err)
		s.Equal("my-service-grafana", ls.Labels()["__name__"])
		s.Equal("my-service-grafana", ls.Labels()["service_name"])
		w.WriteHeader(http.StatusOK)
	})

	argument := Arguments{
		Endpoints: []*EndpointOptions{s.newEndpoint(server, nil)},
	}

	s.newComponent(argument)

	profile := s.newProfile(map[string]string{
		"__name__":     "original-name",
		"service_name": "my-service-grafana",
	})

	err := s.export.Receiver.Appender().AppendIngest(s.ctx, profile)
	s.NoError(err)
	s.Equal(int32(1), s.requestCount.Load())
}

func (s *AppendIngestTestSuite) TestInvalidLabels() {
	server := s.newServer(nil)
	argument := Arguments{
		Endpoints: []*EndpointOptions{s.newEndpoint(server, nil)},
	}

	s.newComponent(argument)

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
		s.Run(tc.name, func() {
			profile := &pyroscope.IncomingProfile{
				RawBody: []byte("test-data"),
				Labels:  tc.labels,
				URL:     &url.URL{Path: "/ingest"},
			}

			err := s.export.Receiver.Appender().AppendIngest(s.ctx, profile)

			if tc.wantErr {
				s.Error(err)
				s.Contains(err.Error(), tc.errMsg)
			} else {
				s.NoError(err)
			}
		})
	}
}

func Test_Write_AppendIngest(t *testing.T) {
	suite.Run(t, new(AppendIngestTestSuite))
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
	c, err := New(
		pyrotestlogger.TestLogger(t),
		noop.Tracer{},
		prometheus.NewRegistry(),
		func(e Exports) {
			defer wg.Done()
			export = e
		},
		"Alloy/239",
		"",
		t.TempDir(),
		argument,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
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
			err = export.Receiver.Appender().Append(t.Context(), tc.labels, []*pyroscope.RawSample{
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
