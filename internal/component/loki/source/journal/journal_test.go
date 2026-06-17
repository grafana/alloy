//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestJournal(t *testing.T) {
	// Create opts for component
	tmp := t.TempDir()
	lr := loki.NewLogsReceiver()
	c, err := New(component.Options{
		ID:         "loki.source.journal.test",
		Logger:     logging.NewSlogNop(),
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

func BenchmarkJournal(b *testing.B) {
	ctrl, err := componenttest.NewControllerFromID(slog.New(slog.DiscardHandler), "loki.source.journal")
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		ctrl.Run(ctx, Arguments{
			ForwardTo: []loki.LogsReceiver{loki.NewLogsReceiver(loki.WithChannel(make(chan loki.Entry, 10000)))},
			Labels:    map[string]string{"test": "yay"},
		}, func(opts component.Options) component.Options {
			opts.DataPath = b.TempDir()
			return opts
		})
		cancel()
	}
}
