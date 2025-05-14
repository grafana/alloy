package gelf

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestGelf(t *testing.T) {
	opts := component.Options{
		Logger:        util.TestAlloyLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	testMsg := `{"version":"1.1","host":"example.org","short_message":"A short message","timestamp":1231231123,"level":5,"_some_extra":"extra"}`
	ch1 := loki.NewLogsReceiver()

	udpListenerAddr := componenttest.GetFreeAddr(t)
	args := Arguments{
		ListenAddress: udpListenerAddr,
		Receivers:     []loki.LogsReceiver{ch1},
	}
	c, err := New(opts, args)
	ctx := t.Context()
	ctx, cancelFunc := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFunc()
	go c.Run(ctx)
	require.NoError(t, err)
	wr, err := net.Dial("udp", udpListenerAddr)
	require.NoError(t, err)
	_, err = wr.Write([]byte(testMsg))
	require.NoError(t, err)
	found := false
	select {
	case <-ctx.Done():
		// If this is called then it failed.
		require.True(t, false)
	case e := <-ch1.Chan():
		require.True(t, strings.Contains(e.Entry.Line, "A short message"))
		found = true
	}
	require.True(t, found)
}
