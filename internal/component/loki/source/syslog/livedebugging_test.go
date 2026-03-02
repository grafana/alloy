package syslog

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
)

type hostStub struct {
	service.Host
	cid string
	c   *Component
}

func newHostStub(cid string, c *Component) *hostStub {
	return &hostStub{
		Host: nil,
		c:    c,
		cid:  cid,
	}
}

func (stub *hostStub) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	if id.LocalID != stub.cid {
		return nil, fmt.Errorf("unexpected component %q (want: %q)", id.LocalID, stub.cid)
	}

	return &component.Info{
		Component:            stub.c,
		LiveDebuggingEnabled: true,
	}, nil
}

const liveDebuggingMsgCountEnvVar = "TEST_SYSLOG_LIVEDEBUGGING_MSG_COUNT"

func getTestMessagesCount(t *testing.T, defaults int) int {
	v, ok := os.LookupEnv(liveDebuggingMsgCountEnvVar)
	if !ok {
		return defaults
	}

	n, err := strconv.Atoi(v)
	require.NoErrorf(t, err, "invalid value in environment variable %q", liveDebuggingMsgCountEnvVar)
	return n
}

func TestLiveDebugging(t *testing.T) {
	const cid = "loki.source.syslog"
	_, isDbgDisabled := os.LookupEnv("TEST_SYSLOG_LIVEDEBUGGING_DISABLED")

	sender := livedebugging.NewLiveDebugging()
	sender.SetEnabled(!isDbgDisabled)
	opts := component.Options{
		ID:            cid,
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
		GetServiceData: func(name string) (any, error) {
			require.Equal(t, livedebugging.ServiceName, name)
			return sender, nil
		},
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

	debugMsgCount := &atomic.Uint32{}
	if !isDbgDisabled {
		cbID := livedebugging.CallbackID("15a4449a-c3db-41c3-824d-9bdc3f31e312")
		h := newHostStub(cid, c)
		err := sender.AddCallback(h, cbID, cid, func(d livedebugging.Data) {
			t.Log(d.DataFunc())
			debugMsgCount.Add(1)
		})
		require.NoError(t, err, "AddCallback failed")
	}

	sendCount := getTestMessagesCount(t, 3)
	lines := make([]string, 0, sendCount)
	for i := range sendCount {
		msg := `<165>1 2023-01-05T09:13:17.001Z host1 app - id1 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"][examplePriority@32473 class="high"] log entry ` + strconv.Itoa(i)
		lines = append(lines, msg)
	}

	var origStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&origStats)

	ctx := t.Context()
	go c.Run(ctx)
	time.Sleep(200 * time.Millisecond)

	// Create and send a Syslog message over TCP to the first listener.
	con, err := net.Dial(syslogtarget.ProtocolTCP, tcpListenerAddr)
	require.NoError(t, err)

	for _, msg := range lines {
		err := writeMessageToStream(con, msg, fmtNewline)
		require.NoError(t, err)
	}
	err = con.Close()
	require.NoError(t, err)

	receivedCount := 0
	for gotCount := 0; gotCount < len(lines); gotCount++ {
		select {
		case <-ch1.Chan():
			receivedCount++
		case <-ctx.Done():
			return
		}
	}

	require.Equal(t, sendCount, receivedCount, "send and received message count mismatch")
	if !isDbgDisabled {
		require.Equal(t, receivedCount, int(debugMsgCount.Load()), "invalid number of log messages")
	}

	finalize(origStats)
}

func finalize(origStats runtime.MemStats) {
	var gotStats runtime.MemStats
	runtime.ReadMemStats(&gotStats)

	fmt.Println("== MEMSTATS ==")
	fmt.Printf("TotalAlloc: %d\n", gotStats.TotalAlloc-origStats.TotalAlloc)
	fmt.Printf("Allocs:     %d\n", gotStats.Alloc-origStats.Alloc)
	fmt.Printf("Mallocs:    %d\n", gotStats.Mallocs-origStats.Mallocs)
}
