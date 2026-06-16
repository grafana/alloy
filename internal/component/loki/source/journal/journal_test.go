//go:build linux && cgo && promtail_journal_enabled

package journal

import (
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

		ctrl.WaitRunning(time.Minute)
		const numMessages = 10

		for i := range numMessages {
			err = cjournal.Send(strconv.Itoa(i), cjournal.PriInfo, map[string]string{
				"ALLOY_TEST_ID": id.String(),
			})
			require.NoError(t, err)
		}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			require.Len(c, collector.Received(), numMessages)
			for i := range collector.Received() {
				require.Equal(c, strconv.Itoa(i), collector.Received()[i].Line)
			}
		}, 5*time.Second, 100*time.Millisecond)

		expectedMetrics := `# HELP loki_source_journal_target_lines_total Total number of successful journal lines read
	# TYPE loki_source_journal_target_lines_total counter
	loki_source_journal_target_lines_total 10
	`
		err = testutil.GatherAndCompare(ctrl.PromRegistry, strings.NewReader(expectedMetrics))
		require.NoError(t, err)
	})
}
