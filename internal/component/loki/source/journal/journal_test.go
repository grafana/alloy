//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	cjournal "github.com/coreos/go-systemd/v22/journal"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
)

func TestJournal(t *testing.T) {
	t.Run("consumes journal messages", func(t *testing.T) {
		ctrl, err := componenttest.NewControllerFromID(slog.New(slog.DiscardHandler), "loki.source.journal")
		require.NoError(t, err)

		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		id := uuid.New()

		go func() {
			_ = ctrl.Run(t.Context(), Arguments{
				ForwardTo: []loki.LogsReceiver{collector.Receiver()},
				MaxAge:    7 * time.Hour,
				Matches:   "ALLOY_TEST_ID=" + id.String(),
			})
		}()

		require.NoError(t, ctrl.WaitRunning(time.Minute))
		const numMessages = 10

		for i := range numMessages {
			err = cjournal.Send(strconv.Itoa(i), cjournal.PriInfo, map[string]string{
				"ALLOY_TEST_ID": id.String(),
			})
			require.NoError(t, err)
		}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			require.Len(c, collector.Received(), numMessages)
		}, 5*time.Second, 100*time.Millisecond)

		for i := range collector.Received() {
			require.Equal(t, strconv.Itoa(i), collector.Received()[i].Line)
		}

		expectedMetrics := `# HELP loki_source_journal_target_lines_total Total number of successful journal lines read
	# TYPE loki_source_journal_target_lines_total counter
	loki_source_journal_target_lines_total 10
	`
		err = testutil.GatherAndCompare(ctrl.PromRegistry, strings.NewReader(expectedMetrics))
		require.NoError(t, err)
	})
}

func BenchmarkJournal(b *testing.B) {
	ctrl, err := componenttest.NewControllerFromID(slog.New(slog.DiscardHandler), "loki.source.journal")
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
