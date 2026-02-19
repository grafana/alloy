package syslog

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	scrapeconfig "github.com/grafana/alloy/internal/component/loki/source/syslog/config"
	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
)

func Test(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	args := Arguments{}
	tcpListenerAddr, udpListenerAddr := componenttest.GetFreeAddr(t), componenttest.GetFreeAddr(t)

	l1 := DefaultListenerConfig
	l1.ListenAddress = tcpListenerAddr
	l1.ListenProtocol = syslogtarget.ProtocolTCP
	l1.Labels = map[string]string{"protocol": syslogtarget.ProtocolTCP}

	l2 := DefaultListenerConfig
	l2.ListenAddress = udpListenerAddr
	l2.ListenProtocol = syslogtarget.ProtocolUDP
	l2.Labels = map[string]string{"protocol": syslogtarget.ProtocolUDP}

	args.SyslogListeners = []ListenerConfig{l1, l2}
	args.ForwardTo = []loki.LogsReceiver{ch1, ch2}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	go c.Run(t.Context())
	time.Sleep(200 * time.Millisecond)

	// Create and send a Syslog message over TCP to the first listener.
	msg := `<165>1 2023-01-05T09:13:17.001Z host1 app - id1 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"][examplePriority@32473 class="high"] An application event log entry...`
	con, err := net.Dial(syslogtarget.ProtocolTCP, tcpListenerAddr)
	require.NoError(t, err)
	err = writeMessageToStream(con, msg, fmtNewline)
	require.NoError(t, err)
	err = con.Close()
	require.NoError(t, err)

	wantLabelSet := model.LabelSet{"protocol": model.LabelValue(syslogtarget.ProtocolTCP)}

	for i := 0; i < 2; i++ {
		select {
		case logEntry := <-ch1.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "An application event log entry...", logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case logEntry := <-ch2.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "An application event log entry...", logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}
	}

	// Send a Syslog message over UDP to the second listener.
	con, err = net.Dial(syslogtarget.ProtocolUDP, udpListenerAddr)
	require.NoError(t, err)
	err = writeMessageToStream(con, msg, fmtOctetCounting)
	require.NoError(t, err)
	err = con.Close()
	require.NoError(t, err)

	wantLabelSet = model.LabelSet{"protocol": "udp"}

	for i := 0; i < 2; i++ {
		select {
		case logEntry := <-ch1.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "An application event log entry...", logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case logEntry := <-ch2.Chan():
			require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
			require.Equal(t, "An application event log entry...", logEntry.Line)
			require.Equal(t, wantLabelSet, logEntry.Labels)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "failed waiting for log line")
		}
	}
}

func TestWithRelabelRules(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	ch1 := loki.NewLogsReceiver()
	args := Arguments{}
	tcpListenerAddr := componenttest.GetFreeAddr(t)

	l := DefaultListenerConfig
	l.ListenAddress = tcpListenerAddr
	l.Labels = map[string]string{"protocol": syslogtarget.ProtocolTCP}

	args.SyslogListeners = []ListenerConfig{l}
	args.ForwardTo = []loki.LogsReceiver{ch1}

	// Create a handler which will be used to retrieve relabeling rules.
	args.RelabelRules = []*alloy_relabel.Config{
		{
			SourceLabels: []string{"__name__"},
			Regex:        mustNewRegexp("__syslog_(.*)"),
			Action:       alloy_relabel.LabelMap,
			Replacement:  "syslog_${1}",
		},
		{
			Regex:  mustNewRegexp("syslog_connection_hostname"),
			Action: alloy_relabel.LabelDrop,
		},
	}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	go c.Run(t.Context())
	time.Sleep(200 * time.Millisecond)

	// Create and send a Syslog message over TCP to the first listener.
	msg := `<165>1 2023-01-05T09:13:17.001Z host1 app - id1 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"][examplePriority@32473 class="high"] An application event log entry...`
	con, err := net.Dial(syslogtarget.ProtocolTCP, tcpListenerAddr)
	require.NoError(t, err)
	err = writeMessageToStream(con, msg, fmtNewline)
	require.NoError(t, err)
	err = con.Close()
	require.NoError(t, err)

	// The entry should've had the relabeling rules applied to it.
	wantLabelSet := model.LabelSet{
		"protocol":                     model.LabelValue(syslogtarget.ProtocolTCP),
		"syslog_connection_ip_address": "127.0.0.1",
		"syslog_message_app_name":      "app",
		"syslog_message_facility":      "local4",
		"syslog_message_hostname":      "host1",
		"syslog_message_msg_id":        "id1",
		"syslog_message_severity":      "notice",
	}

	select {
	case logEntry := <-ch1.Chan():
		require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
		require.Equal(t, "An application event log entry...", logEntry.Line)
		require.Equal(t, wantLabelSet, logEntry.Labels)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "failed waiting for log line")
	}
}

func writeMessageToStream(w io.Writer, msg string, formatter formatFunc) error {
	_, err := fmt.Fprint(w, formatter(msg))
	if err != nil {
		return err
	}
	return nil
}

type formatFunc func(string) string

var (
	fmtOctetCounting = func(s string) string { return fmt.Sprintf("%d %s", len(s), s) }
	fmtNewline       = func(s string) string { return s + "\n" }
)

func mustNewRegexp(s string) alloy_relabel.Regexp {
	re, err := regexp.Compile("^(?:" + s + ")$")
	if err != nil {
		panic(err)
	}
	return alloy_relabel.Regexp{Regexp: re}
}

func TestShutdownAndRebindOnSamePort(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	addr := componenttest.GetFreeAddr(t)

	// Create and start the first component listening on addr over TCP.
	ch1 := loki.NewLogsReceiver()
	args1 := Arguments{}
	l1 := DefaultListenerConfig
	l1.ListenAddress = addr
	l1.ListenProtocol = syslogtarget.ProtocolTCP
	l1.Labels = map[string]string{"phase": "first"}
	args1.SyslogListeners = []ListenerConfig{l1}
	args1.ForwardTo = []loki.LogsReceiver{ch1}

	c1, err := New(opts, args1)
	require.NoError(t, err)

	ctx1, cancel1 := context.WithCancel(context.Background())
	done1 := make(chan error, 1)
	go func() { done1 <- c1.Run(ctx1) }()
	time.Sleep(200 * time.Millisecond)

	// Create the second component on the same addr, don't start yet.
	ch2 := loki.NewLogsReceiver()
	args2 := Arguments{}
	l2 := DefaultListenerConfig
	l2.ListenAddress = addr
	l2.ListenProtocol = syslogtarget.ProtocolTCP
	l2.Labels = map[string]string{"phase": "second"}
	args2.SyslogListeners = []ListenerConfig{l2}
	args2.ForwardTo = []loki.LogsReceiver{ch2}

	c2, err := New(opts, args2)
	require.NoError(t, err)

	// Stop first component and wait for shutdown to release the port.
	cancel1()
	select {
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timeout waiting for first component to stop")
	case err := <-done1:
		require.NoError(t, err)
	}

	// Run the second component
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan error, 1)
	go func() { done2 <- c2.Run(ctx2) }()

	// Send a syslog message to verify the second component successfully bound.
	msg := `<165>1 2023-01-05T09:13:17.001Z host1 app - id1 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"][examplePriority@32473 class="high"] Rebind successful`

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		conn, err := net.Dial(syslogtarget.ProtocolTCP, addr)
		require.NoError(collect, err)
		require.NoError(collect, writeMessageToStream(conn, msg, fmtNewline))
		require.NoError(collect, conn.Close())
	}, 5*time.Second, 10*time.Millisecond, "failed to dial and write message to stream")

	select {
	case logEntry := <-ch2.Chan():
		require.WithinDuration(t, time.Now(), logEntry.Timestamp, 1*time.Second)
		require.Equal(t, "Rebind successful", logEntry.Line)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "did not receive log from second component; port may not have been released")
	}

	// Cleanup second component.
	cancel2()
	select {
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timeout waiting for second component to stop")
	case err := <-done2:
		require.NoError(t, err)
	}
}

func TestExperimentalFeaturesStabilityLevel(t *testing.T) {
	cases := []struct {
		label     string
		expectErr string
		setCfg    func(*ListenerConfig)
	}{
		{
			label:     "syslog-raw-format",
			expectErr: "syslog format is available only at experimental stability level",
			setCfg: func(lc *ListenerConfig) {
				lc.SyslogFormat = scrapeconfig.SyslogFormatRaw
			},
		},
		{
			label:     "cisco-ios",
			expectErr: "rfc3164_cisco_components block is available only at experimental stability level",
			setCfg: func(lc *ListenerConfig) {
				lc.SyslogFormat = scrapeconfig.SyslogFormatRFC3164
				lc.RFC3164CiscoComponents = &RFC3164CiscoComponents{EnableAll: true}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			opts := component.Options{
				Logger:        util.TestAlloyLogger(t),
				Registerer:    prometheus.NewRegistry(),
				OnStateChange: func(e component.Exports) {},
				MinStability:  featuregate.StabilityGenerallyAvailable,
			}

			lc := DefaultListenerConfig
			lc.ListenAddress = "127.0.0.1:1234"
			lc.ListenProtocol = syslogtarget.ProtocolTCP
			tc.setCfg(&lc)

			rcv := loki.NewLogsReceiver()
			args := Arguments{
				SyslogListeners: []ListenerConfig{lc},
				ForwardTo:       []loki.LogsReceiver{rcv},
			}

			// Check if requires experimental level
			_, err := New(opts, args)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectErr, "component should require experimental level")

			// Check if there is no error when stability level is experimental
			opts.MinStability = featuregate.StabilityExperimental
			_, err = New(opts, args)
			require.NoError(t, err, "feature should work at experimental level")
		})
	}
}

func TestLiveDebuggingServiceWorks(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
		GetServiceData: func(name string) (any, error) {
			require.Equal(t, name, livedebugging.ServiceName)
			return livedebugging.NewLiveDebugging(), nil
		},
	}

	l1 := DefaultListenerConfig
	l1.ListenAddress = "0.0.0.0:1234"
	l1.ListenProtocol = syslogtarget.ProtocolTCP

	args := Arguments{
		SyslogListeners: []ListenerConfig{l1},
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	require.NotNil(t, c.liveDbgListener)
	_, ok := c.liveDbgListener.(*liveDebuggingWriter)
	require.True(t, ok, "expected livedebugging to work")
}
