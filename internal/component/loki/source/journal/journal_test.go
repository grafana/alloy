//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestJournal(t *testing.T) {
	// Create opts for component
	tmp := t.TempDir()
	lr := loki.NewLogsReceiver()
	c, err := New(component.Options{
		ID:         "loki.source.journal.test",
		Logger:     util.TestAlloyLogger(t),
		DataPath:   tmp,
		Registerer: prometheus.DefaultRegisterer,
	}, Arguments{
		FormatAsJson: false,
		MaxAge:       7 * time.Hour,
		Path:         "",
		Receivers:    []loki.LogsReceiver{lr},
	})
	require.NoError(t, err)
	ctx := t.Context()
	ctx, cnc := context.WithTimeout(ctx, 5*time.Second)
	defer cnc()
	go c.Run(ctx)
	ts := time.Now().String()
	err = journal.Send(ts, journal.PriInfo, nil)
	require.NoError(t, err)
	found := false
	for !found {
		select {
		case <-ctx.Done():
			found = true
			// Timed out getting message
			require.True(t, false)
		case msg := <-lr.Chan():
			if strings.Contains(msg.Line, ts) {
				found = true
				break
			}
		}
	}
	require.True(t, found)
}
