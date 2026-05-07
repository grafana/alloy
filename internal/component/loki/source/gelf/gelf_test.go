package gelf

import (
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestGelf(t *testing.T) {
	opts := component.Options{
		Logger:        logging.NewSlogNop(),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}

	testMsg := `{"version":"1.1","host":"example.org","short_message":"A short message","timestamp":1231231123,"level":5,"_some_extra":"extra"}`
	collector := loki.NewCollectingConsumer()

	udpListenerAddr := componenttest.GetFreeAddr(t)
	args := Arguments{
		ListenAddress: udpListenerAddr,
		ForwardTo:     []loki.Consumer{collector},
	}
	c, err := New(opts, args)
	require.NoError(t, err)

	go c.Run(t.Context())

	wr, err := net.Dial("udp", udpListenerAddr)
	require.NoError(t, err)
	_, err = wr.Write([]byte(testMsg))
	require.NoError(t, err)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.Len(c, collector.Entries(), 1)
	}, 5*time.Second, 100*time.Millisecond)

	got := collector.Entries()[0]
	require.Contains(t, got.Line, "A short message")
}
