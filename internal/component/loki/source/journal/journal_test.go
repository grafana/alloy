//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestJournal(t *testing.T) {
	// Create opts for component
	tmp := t.TempDir()
	lr := loki.NewLogsReceiver()
	c, err := New(component.Options{
		ID:         "loki.source.journal.test",
		Logger:     log.NewNopLogger(),
		DataPath:   tmp,
		Registerer: prometheus.DefaultRegisterer,
	}, Arguments{
		FormatAsJson: false,
		MaxAge:       7 * time.Hour,
		Path:         "",
		ForwardTo:    []loki.LogsReceiver{lr},
	})
	require.NoError(t, err)
	ctx := t.Context()
	ctx, cnc := context.WithTimeout(ctx, 5*time.Second)
	defer cnc()
	go c.Run(ctx)
	ts := time.Now().String()
	err = journal.Send(ts, journal.PriInfo, nil)
	require.NoError(t, err)

	select {
	case <-ctx.Done():
		t.Error("did not get entry in time")
		return
	case msg := <-lr.Chan():
		if strings.Contains(msg.Line, ts) {
			return
		}
	}
}
