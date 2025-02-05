package receive_http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

func TestForwardsMetrics(t *testing.T) {
	timestamp := time.Now().Add(time.Second).UnixMilli()
	input := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 12},
			{Timestamp: timestamp + 1, Value: 24},
			{Timestamp: timestamp + 2, Value: 48},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 191},
			{Timestamp: timestamp + 1, Value: 1337},
		},
	}}

	expected := []testSample{
		{ts: timestamp, val: 12, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 1, val: 24, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 2, val: 48, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp, val: 191, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
		{ts: timestamp + 1, val: 1337, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	actualSamples := make(chan testSample, 100)

	// Start the component
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    port,
			},
			GRPC: testGRPCConfig(t),
		},
		ForwardTo: testAppendable(actualSamples),
	}
	comp, err := New(testOptions(t), args)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	verifyExpectations(t, input, expected, actualSamples, args, ctx)
}

func TestUpdate(t *testing.T) {
	timestamp := time.Now().Add(time.Second).UnixMilli()
	input01 := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 12},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 191},
		},
	}}
	expected01 := []testSample{
		{ts: timestamp, val: 12, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp, val: 191, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	input02 := []prompb.TimeSeries{{
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "foo", Value: "bar"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp + 1, Value: 24},
			{Timestamp: timestamp + 2, Value: 48},
		},
	}, {
		Labels: []prompb.Label{{Name: "__name__", Value: "test_metric"}, {Name: "cluster", Value: "local"}, {Name: "fizz", Value: "buzz"}},
		Samples: []prompb.Sample{
			{Timestamp: timestamp + 1, Value: 1337},
		},
	}}
	expected02 := []testSample{
		{ts: timestamp + 1, val: 24, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 2, val: 48, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "foo", "bar")},
		{ts: timestamp + 1, val: 1337, l: labels.FromStrings("__name__", "test_metric", "cluster", "local", "fizz", "buzz")},
	}

	actualSamples := make(chan testSample, 100)

	// Start the component
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	args := Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    port,
			},
			GRPC: testGRPCConfig(t),
		},
		ForwardTo: testAppendable(actualSamples),
	}
	comp, err := New(testOptions(t), args)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		require.NoError(t, comp.Run(ctx))
	}()

	verifyExpectations(t, input01, expected01, actualSamples, args, ctx)

	otherPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	args = Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    otherPort,
			},
			GRPC: testGRPCConfig(t),
		},
		ForwardTo: testAppendable(actualSamples),
	}
	err = comp.Update(args)
	require.NoError(t, err)

	verifyExpectations(t, input02, expected02, actualSamples, args, ctx)
}

func testGRPCConfig(t *testing.T) *fnet.GRPCConfig {
	return &fnet.GRPCConfig{ListenAddress: "127.0.0.1", ListenPort: getFreePort(t)}
}

func TestServerRestarts(t *testing.T) {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)

	otherPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	testCases := []struct {
		name          string
		initialArgs   Arguments
		newArgs       Arguments
		shouldRestart bool
	}{
		{
			name: "identical args require no restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				ForwardTo: []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				ForwardTo: []storage.Appendable{},
			},
			shouldRestart: false,
		},
		{
			name: "forward_to update does not require restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				ForwardTo: []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				ForwardTo: testAppendable(nil),
			},
			shouldRestart: false,
		},
		{
			name: "hostname change requires restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				ForwardTo: []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "127.0.0.1", ListenPort: port},
				},
				ForwardTo: testAppendable(nil),
			},
			shouldRestart: true,
		},
		{
			name: "port change requires restart",
			initialArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: port},
				},
				ForwardTo: []storage.Appendable{},
			},
			newArgs: Arguments{
				Server: &fnet.ServerConfig{
					HTTP: &fnet.HTTPConfig{ListenAddress: "localhost", ListenPort: otherPort},
				},
				ForwardTo: testAppendable(nil),
			},
			shouldRestart: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			comp, err := New(testOptions(t), tc.initialArgs)
			require.NoError(t, err)

			serverExit := make(chan error)
			go func() {
				serverExit <- comp.Run(ctx)
			}()

			waitForServerToBeReady(t, comp.args)

			initialServer := comp.server
			require.NotNil(t, initialServer)

			err = comp.Update(tc.newArgs)
			require.NoError(t, err)

			waitForServerToBeReady(t, comp.args)

			require.NotNil(t, comp.server)
			restarted := initialServer != comp.server

			require.Equal(t, tc.shouldRestart, restarted)

			// shut down cleanly to release ports for other tests
			cancel()
			select {
			case err := <-serverExit:
				require.NoError(t, err, "unexpected error on server exit")
			case <-time.After(5 * time.Second):
				t.Fatalf("timed out waiting for server to shut down")
			}
		})
	}
}

type testSample struct {
	ts  int64
	val float64
	l   labels.Labels
}

func waitForServerToBeReady(t *testing.T, args Arguments) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := http.Get(fmt.Sprintf(
			"http://%v:%d/wrong/path",
			args.Server.HTTP.ListenAddress,
			args.Server.HTTP.ListenPort,
		))
		t.Logf("err: %v, resp: %v", err, resp)
		assert.Nil(c, err)
		assert.Equal(c, 404, resp.StatusCode)
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func verifyExpectations(
	t *testing.T,
	input []prompb.TimeSeries,
	expected []testSample,
	actualSamples chan testSample,
	args Arguments,
	ctx context.Context,
) {
	// In case server didn't start yet
	waitForServerToBeReady(t, args)

	// Send the input time series to the component
	endpoint := fmt.Sprintf(
		"http://%s:%d/api/v1/metrics/write",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	)
	err := request(ctx, endpoint, &prompb.WriteRequest{Timeseries: input})
	require.NoError(t, err)

	// Verify we receive expected metrics
	for _, exp := range expected {
		select {
		case actual := <-actualSamples:
			require.Equal(t, exp, actual)
		case <-ctx.Done():
			t.Fatalf("test timed out")
		}
	}

	select {
	case unexpected := <-actualSamples:
		t.Fatalf("unexpected extra sample received: %v", unexpected)
	default:
	}
}

func testAppendable(actualSamples chan testSample) []storage.Appendable {
	hookFn := func(
		ref storage.SeriesRef,
		l labels.Labels,
		ts int64,
		val float64,
		next storage.Appender,
	) (storage.SeriesRef, error) {

		actualSamples <- testSample{ts: ts, val: val, l: l}
		return ref, nil
	}

	ls := labelstore.New(nil, prometheus.DefaultRegisterer)
	return []storage.Appendable{alloyprom.NewInterceptor(
		nil,
		ls,
		alloyprom.WithAppendHook(
			hookFn))}
}

func request(ctx context.Context, rawRemoteWriteURL string, req *prompb.WriteRequest) error {
	remoteWriteURL, err := url.Parse(rawRemoteWriteURL)
	if err != nil {
		return err
	}

	client, err := remote.NewWriteClient("remote-write-client", &remote.ClientConfig{
		URL:     &config.URL{URL: remoteWriteURL},
		Timeout: model.Duration(30 * time.Second),
	})
	if err != nil {
		return err
	}

	buf, err := proto.Marshal(protoadapt.MessageV2Of(req))
	if err != nil {
		return err
	}

	compressed := snappy.Encode(buf, buf)
	_, err = client.Store(ctx, compressed, 0)
	return err
}

func testOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "prometheus.receive_http.test",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		GetServiceData: func(name string) (interface{}, error) {
			return labelstore.New(nil, prometheus.DefaultRegisterer), nil
		},
	}
}

func getFreePort(t *testing.T) int {
	p, err := freeport.GetFreePort()
	require.NoError(t, err)
	return p
}
