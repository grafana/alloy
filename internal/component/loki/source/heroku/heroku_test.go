package heroku

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/heroku/internal/herokutarget"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/regexp"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestPush(t *testing.T) {
	opts := defaultOptions(t)

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	args := testArgsWith(t, func(args *Arguments) {
		args.ForwardTo = []loki.LogsReceiver{ch1, ch2}
		args.RelabelRules = rulesExport
		args.Labels = map[string]string{"foo": "bar"}
	})
	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	go func() { require.NoError(t, c.Run(t.Context())) }()
	waitForServerToBeReady(t, c)

	// Create a Heroku Drain Request and send it to the launched server.
	req, err := http.NewRequest(http.MethodPost, getEndpoint(c.target), strings.NewReader(testPayload))
	require.NoError(t, err)

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, res.StatusCode)

	// Check the received log entries
	wantLabelSet := model.LabelSet{"foo": "bar", "host": "host", "app": "heroku", "proc": "router", "log_id": "-"}
	wantLogLine := "at=info method=GET path=\"/\" host=cryptic-cliffs-27764.herokuapp.com request_id=59da6323-2bc4-4143-8677-cc66ccfb115f fwd=\"181.167.87.140\" dyno=web.1 connect=0ms service=3ms status=200 bytes=6979 protocol=https\n"

	for range 2 {
		select {
		case logEntry := <-ch1.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, wantLogLine, logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case logEntry := <-ch2.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, wantLogLine, logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}
	}
}

func TestUpdate_detectsWhenTargetRequiresARestart(t *testing.T) {
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
				args.Server.HTTP.ListenAddress = "127.0.0.1"
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
			name: "change in labels requires server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.Labels = map[string]string{"some": "label"}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: true,
		},
		{
			name: "change in relabel rules requires server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.RelabelRules = alloy_relabel.Rules{}
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: true,
		},
		{
			name: "change in use incoming timestamp requires server restart",
			args: testArgsWithPorts(httpPort, grpcPort),
			newArgs: testArgsWith(t, func(args *Arguments) {
				args.UseIncomingTimestamp = !args.UseIncomingTimestamp
				args.Server.HTTP.ListenPort = httpPort
				args.Server.GRPC.ListenPort = grpcPort
			}),
			restartRequired: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			comp, err := New(
				defaultOptions(t),
				tc.args,
			)
			require.NoError(t, err)
			defer func() {
				// in order to cleanly shutdown, we want to make sure the server is running first.
				waitForServerToBeReady(t, comp)
				require.NoError(t, comp.target.Stop())
			}()

			// in order to cleanly update, we want to make sure the server is running first.
			waitForServerToBeReady(t, comp)

			targetBefore := comp.target
			err = comp.Update(tc.newArgs)
			require.NoError(t, err)

			restarted := targetBefore != comp.target
			require.Equal(t, restarted, tc.restartRequired)
		})
	}
}

const testPayload = `270 <158>1 2022-06-13T14:52:23.622778+00:00 host heroku router - at=info method=GET path="/" host=cryptic-cliffs-27764.herokuapp.com request_id=59da6323-2bc4-4143-8677-cc66ccfb115f fwd="181.167.87.140" dyno=web.1 connect=0ms service=3ms status=200 bytes=6979 protocol=https
`

var rulesExport = alloy_relabel.Rules{
	{
		SourceLabels: []string{"__heroku_drain_host"},
		Regex:        newRegexp(),
		Action:       alloy_relabel.Replace,
		Replacement:  "$1",
		TargetLabel:  "host",
	},
	{
		SourceLabels: []string{"__heroku_drain_app"},
		Regex:        newRegexp(),
		Action:       alloy_relabel.Replace,
		Replacement:  "$1",
		TargetLabel:  "app",
	},
	{
		SourceLabels: []string{"__heroku_drain_proc"},
		Regex:        newRegexp(),
		Action:       alloy_relabel.Replace,
		Replacement:  "$1",
		TargetLabel:  "proc",
	},
	{
		SourceLabels: []string{"__heroku_drain_log_id"},
		Regex:        newRegexp(),
		Action:       alloy_relabel.Replace,
		Replacement:  "$1",
		TargetLabel:  "log_id",
	},
}

func defaultOptions(t *testing.T) component.Options {
	return component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}
}

func testArgsWithPorts(httpPort int, grpcPort int) Arguments {
	return Arguments{
		Server: &fnet.ServerConfig{
			HTTP: &fnet.HTTPConfig{
				ListenAddress: "localhost",
				ListenPort:    httpPort,
			},
			GRPC: &fnet.GRPCConfig{
				ListenAddress: "localhost",
				ListenPort:    grpcPort,
			},
		},
		ForwardTo: []loki.LogsReceiver{loki.NewLogsReceiver(), loki.NewLogsReceiver()},
		Labels:    map[string]string{"foo": "bar", "fizz": "buzz"},
		RelabelRules: alloy_relabel.Rules{
			{
				SourceLabels: []string{"tag"},
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("ignore")},
				Action:       alloy_relabel.Drop,
			},
		},
		UseIncomingTimestamp: false,
	}
}

func testArgsWith(t *testing.T, mutator func(arguments *Arguments)) Arguments {
	a := testArgsWithPorts(getFreePort(t), getFreePort(t))
	mutator(&a)
	return a
}

func waitForServerToBeReady(t *testing.T, comp *Component) {
	require.Eventuallyf(t, func() bool {
		resp, err := http.Get(fmt.Sprintf(
			"http://%v/wrong/url",
			comp.target.HTTPListenAddress(),
		))
		return err == nil && resp.StatusCode == 404
	}, 5*time.Second, 20*time.Millisecond, "server failed to start before timeout")
}

func getFreePort(t *testing.T) int {
	port, err := freeport.GetFreePort()
	require.NoError(t, err)
	return port
}

func newRegexp() alloy_relabel.Regexp {
	re, err := regexp.Compile("^(?:(.*))$")
	if err != nil {
		panic(err)
	}
	return alloy_relabel.Regexp{Regexp: re}
}

func getEndpoint(target *herokutarget.HerokuTarget) string {
	return fmt.Sprintf("http://%s%s", target.HTTPListenAddress(), target.DrainEndpoint())
}
