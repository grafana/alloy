package syslog

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget"
	"github.com/grafana/alloy/internal/util"
)

func TestEscalation(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(component.Exports) {},
	}

	logs := loki.NewLogsReceiver()
	tcpListenerAddr := "127.0.0.1:51898"

	listener := DefaultListenerConfig
	listener.ListenAddress = tcpListenerAddr
	listener.ListenProtocol = syslogtarget.ProtocolTCP
	listener.Labels = map[string]string{
		"job":      "security-monitoring",
		"host":     "secfw-a",
		"source":   "junos",
		"protocol": syslogtarget.ProtocolTCP,
	}
	listener.UseRFC5424Message = true
	listener.UseIncomingTimestamp = true

	args := Arguments{
		SyslogListeners: []ListenerConfig{listener},
		ForwardTo:       []loki.LogsReceiver{logs},
	}

	c, err := New(opts, args)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	var collected []loki.Entry

	for {
		select {
		case entry := <-logs.Chan():
			collected = append(collected, entry)
		case <-ctx.Done():
			require.NoError(t, <-done)
			printCollected(t, collected)
			return
		}
	}
}

func printCollected(t *testing.T, collected []loki.Entry) {
	t.Logf("collected %d log entries", len(collected))
	for _, line := range collected {
		t.Log(line)
	}
}
