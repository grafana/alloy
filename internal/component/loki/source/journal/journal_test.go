//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestJournal(t *testing.T) {
	// Create opts for component
	tmp := t.TempDir()
	collector := loki.NewCollectingConsumer()
	c, err := New(component.Options{
		ID:         "loki.source.journal.test",
		Logger:     logging.NewSlogNop(),
		DataPath:   tmp,
		Registerer: prometheus.DefaultRegisterer,
	}, Arguments{
		FormatAsJson: false,
		MaxAge:       7 * time.Hour,
		Path:         "",
		ForwardTo:    []loki.Consumer{collector},
	})
	require.NoError(t, err)

	go c.Run(t.Context())

	ts := time.Now().String()
	err = journal.Send(ts, journal.PriInfo, nil)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		for _, e := range collector.Entries() {
			if strings.Contains(e.Line, ts) {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond)
}
