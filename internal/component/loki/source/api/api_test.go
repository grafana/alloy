package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/phayes/freeport"

	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client"
	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/util"
)

func TestLokiSourceAPI_Simple(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	receiver := fake.NewClient(func() {})
	defer receiver.Stop()

	args := testArgsWith(t, func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8532
		a.ForwardTo = []loki.LogsReceiver{receiver.LogsReceiver()}
		a.UseIncomingTimestamp = true
	})
	opts := defaultOptions(t)
	_, shutdown := startTestComponent(t, opts, args, ctx)
	defer shutdown()

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	now := time.Now()
	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  logproto.Entry{Timestamp: now, Line: "hello world!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via loki client")
	}

	require.Eventually(
		t,
		func() bool { return len(receiver.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received := receiver.Received()[0]
	assert.Equal(t, received.Line, "hello world!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"source": "test",
		"foo":    "bar",
		"fizz":   "buzz",
	})
}

func TestLokiSourceAPI_Update(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	receiver := fake.NewClient(func() {})
	defer receiver.Stop()

	args := testArgsWith(t, func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8583
		a.ForwardTo = []loki.LogsReceiver{receiver.LogsReceiver()}
		a.UseIncomingTimestamp = true
		a.Labels = map[string]string{"test_label": "before"}
	})
	opts := defaultOptions(t)
	c, shutdown := startTestComponent(t, opts, args, ctx)
	defer shutdown()

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	now := time.Now()
	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  logproto.Entry{Timestamp: now, Line: "hello world!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via loki client")
	}

	require.Eventually(
		t,
		func() bool { return len(receiver.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received := receiver.Received()[0]
	assert.Equal(t, received.Line, "hello world!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"test_label": "before",
		"source":     "test",
	})

	args.Labels = map[string]string{"test_label": "after"}
	err := c.Update(args)
	require.NoError(t, err)

	receiver.Clear()

	select {
	case lokiClient.Chan() <- loki.Entry{
		Labels: map[model.LabelName]model.LabelValue{"source": "test"},
		Entry:  logproto.Entry{Timestamp: now, Line: "hello brave new world!"},
	}:
	case <-ctx.Done():
		t.Fatalf("timed out while sending test entries via loki client")
	}
	require.Eventually(
		t,
		func() bool { return len(receiver.Received()) == 1 },
		5*time.Second,
		10*time.Millisecond,
		"did not receive the forwarded message within the timeout",
	)
	received = receiver.Received()[0]
	assert.Equal(t, received.Line, "hello brave new world!")
	assert.Equal(t, received.Timestamp.Unix(), now.Unix())
	assert.Equal(t, received.Labels, model.LabelSet{
		"test_label": "after",
		"source":     "test",
	})
}

func TestLokiSourceAPI_FanOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const receiversCount = 10
	var receivers = make([]*fake.Client, receiversCount)
	for i := 0; i < receiversCount; i++ {
		receivers[i] = fake.NewClient(func() {})
	}

	args := testArgsWith(t, func(a *Arguments) {
		a.Server.HTTP.ListenPort = 8537
		a.ForwardTo = mapToChannels(receivers)
	})
	opts := defaultOptions(t)

	comp, err := New(opts, args)
	require.NoError(t, err)
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	defer comp.stop()

	lokiClient := newTestLokiClient(t, args, opts)
	defer lokiClient.Stop()

	const messagesCount = 100
	for i := 0; i < messagesCount; i++ {
		entry := loki.Entry{
			Labels: map[model.LabelName]model.LabelValue{"source": "test"},
			Entry:  logproto.Entry{Line: fmt.Sprintf("test message #%d", i)},
		}
		select {
		case lokiClient.Chan() <- entry:
		case <-ctx.Done():
			t.Log("timed out while sending test entries via loki client")
		}
	}

	require.Eventually(
		t,
		func() bool {
			for i := 0; i < receiversCount; i++ {
				if len(receivers[i].Received()) != messagesCount {
					return false
				}
			}
			return true
		},
		5*time.Second,
		10*time.Millisecond,
		"did not receive all the expected messages within the timeout",
	)
}

func TestComponent_detectsWhenUpdateRequiresARestart(t *testing.T) {
	httpPort := getFreePort(t)
	grpcPort := getFreePort(t)
	tests := []struct {
		name            string
		args            Arguments
		newArgs         Arguments
		restartRequired bool
	}{
		{
			name:            "identical args don't require server restart",
			args:            testArgsWithPorts(httpPort, grpcPort),
			newArgs:         testArgsWithPorts(httpPort, grpcPort),
			restartRequired: false,
		},
		{
			name: "change in address requires server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.Server.HTTP.ListenAddress = "localhost"
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: true,
		},
		{
			name:            "change in port requires server restart",
			args:            testArgsWithPorts(httpPort, grpcPort),
			newArgs:         testArgsWithPorts(getFreePort(t), grpcPort),
			restartRequired: true,
		},
		{
			name: "change in forwardTo does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.ForwardTo = []loki.LogsReceiver{}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
		{
			name: "change in labels does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.Labels = map[string]string{"some": "label"}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
		{
			name: "change in relabel rules does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.RelabelRules = relabel.Rules{}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
		{
			name: "change in use incoming timestamp does not require server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.UseIncomingTimestamp = !args.UseIncomingTimestamp
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comp, err := New(
				defaultOptions(t),
				tc.args,
			)
			require.NoError(t, err)

			// in order to cleanly update, we want to make sure the server is running first.
			waitForServerToBeReady(t, comp)

			serverBefore := comp.server
			err = comp.Update(tc.newArgs)
			require.NoError(t, err)

			restarted := serverBefore != comp.server
			assert.Equal(t, restarted, tc.restartRequired)

			// in order to cleanly shutdown, we want to make sure the server is running first.
			waitForServerToBeReady(t, comp)
			comp.stop()
		})
	}
}

func TestDefaultServerConfig(t *testing.T) {
	args := testArgs(t)
	args.Server = nil // user did not define server options

	comp, err := New(
		defaultOptions(t),
		args,
	)
	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		resp, err := http.Get(fmt.Sprintf(
			"http://%v:%d/wrong/url",
			"localhost",
			net.DefaultHTTPPort,
		))
		return err == nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")

	comp.stop()
}

func startTestComponent(
	t *testing.T,
	opts component.Options,
	args Arguments,
	ctx context.Context,
) (component.Component, func()) {

	comp, err := New(opts, args)
	require.NoError(t, err)
	go func() {
		err := comp.Run(ctx)
		require.NoError(t, err)
	}()

	return comp, func() {
		// in order to cleanly shutdown, we want to make sure the server is running first.
		waitForServerToBeReady(t, comp)
		comp.stop()
	}
}

func waitForServerToBeReady(t *testing.T, comp *Component) {
	require.Eventuallyf(t, func() bool {
		resp, err := http.Get(fmt.Sprintf(
			"http://%v:%d/wrong/url",
			comp.server.ServerConfig().HTTP.ListenAddress,
			comp.server.ServerConfig().HTTP.ListenPort,
		))
		return err == nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func mapToChannels(clients []*fake.Client) []loki.LogsReceiver {
	channels := make([]loki.LogsReceiver, len(clients))
	for i := 0; i < len(clients); i++ {
		channels[i] = clients[i].LogsReceiver()
	}
	return channels
}

func newTestLokiClient(t *testing.T, args Arguments, opts component.Options) client.Client {
	url := flagext.URLValue{}
	err := url.Set(fmt.Sprintf(
		"http://%s:%d/api/v1/push",
		args.Server.HTTP.ListenAddress,
		args.Server.HTTP.ListenPort,
	))
	require.NoError(t, err)

	lokiClient, err := client.New(
		client.NewMetrics(nil),
		client.Config{
			URL:     url,
			Timeout: 5 * time.Second,
		},
		0,
		0,
		false,
		opts.Logger,
	)
	require.NoError(t, err)
	return lokiClient
}

func defaultOptions(t *testing.T) component.Options {
	return component.Options{
		ID:         "loki.source.api.test",
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
	}
}

func testArgsWith(t *testing.T, mutator func(arguments *Arguments)) Arguments {
	a := testArgs(t)
	mutator(&a)
	return a
}

func testArgs(t *testing.T) Arguments {
	return testArgsWithPorts(getFreePort(t), getFreePort(t))
}

func testArgsWithPorts(httpPort int, grpcPort int) Arguments {
	return Arguments{
		Server: &net.ServerConfig{
			HTTP: &net.HTTPConfig{
				ListenAddress: "127.0.0.1",
				ListenPort:    httpPort,
			},
			GRPC: &net.GRPCConfig{
				ListenAddress: "127.0.0.1",
				ListenPort:    grpcPort,
			},
		},
		ForwardTo: []loki.LogsReceiver{loki.NewLogsReceiver(), loki.NewLogsReceiver()},
		Labels:    map[string]string{"foo": "bar", "fizz": "buzz"},
		RelabelRules: relabel.Rules{
			{
				SourceLabels: []string{"tag"},
				Regex:        relabel.Regexp{Regexp: regexp.MustCompile("ignore")},
				Action:       relabel.Drop,
			},
		},
		UseIncomingTimestamp: false,
	}
}

func getFreePort(t *testing.T) int {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	return port
}
