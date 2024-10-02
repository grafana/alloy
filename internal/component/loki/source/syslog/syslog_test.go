package syslog

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/regexp"
	"github.com/phayes/freeport"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	ch1, ch2 := loki.NewLogsReceiver(), loki.NewLogsReceiver()
	args := Arguments{}
	tcpListenerAddr, udpListenerAddr := getFreeAddr(t), getFreeAddr(t)

	l1 := DefaultListenerConfig
	l1.ListenAddress = tcpListenerAddr
	l1.ListenProtocol = "tcp"
	l1.Labels = map[string]string{"protocol": "tcp"}

	l2 := DefaultListenerConfig
	l2.ListenAddress = udpListenerAddr
	l2.ListenProtocol = "udp"
	l2.Labels = map[string]string{"protocol": "udp"}

	args.SyslogListeners = []ListenerConfig{l1, l2}
	args.ForwardTo = []loki.LogsReceiver{ch1, ch2}

	// Create and run the component.
	c, err := New(opts, args)
	require.NoError(t, err)

	go c.Run(context.Background())
	time.Sleep(200 * time.Millisecond)

	// Create and send a Syslog message over TCP to the first listener.
	msg := `<165>1 2023-01-05T09:13:17.001Z host1 app - id1 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"][examplePriority@32473 class="high"] An application event log entry...`
	con, err := net.Dial("tcp", tcpListenerAddr)
	require.NoError(t, err)
	writeMessageToStream(con, msg, fmtNewline)
	err = con.Close()
	require.NoError(t, err)

	wantLabelSet := model.LabelSet{"protocol": "tcp"}

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
	con, err = net.Dial("udp", udpListenerAddr)
	require.NoError(t, err)
	writeMessageToStream(con, msg, fmtOctetCounting)
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
	tcpListenerAddr := getFreeAddr(t)

	l := DefaultListenerConfig
	l.ListenAddress = tcpListenerAddr
	l.Labels = map[string]string{"protocol": "tcp"}

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

	go c.Run(context.Background())
	time.Sleep(200 * time.Millisecond)

	// Create and send a Syslog message over TCP to the first listener.
	msg := `<165>1 2023-01-05T09:13:17.001Z host1 app - id1 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"][examplePriority@32473 class="high"] An application event log entry...`
	con, err := net.Dial("tcp", tcpListenerAddr)
	require.NoError(t, err)
	writeMessageToStream(con, msg, fmtNewline)
	err = con.Close()
	require.NoError(t, err)

	// The entry should've had the relabeling rules applied to it.
	wantLabelSet := model.LabelSet{
		"protocol":                     "tcp",
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

func getFreeAddr(t *testing.T) string {
	t.Helper()

	portNumber, err := freeport.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("127.0.0.1:%d", portNumber)
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
