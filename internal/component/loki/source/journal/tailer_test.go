//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/component/loki/source/journal/internal/sdjournal"
)

func TestTailer(t *testing.T) {
	t.Run("parsing error", func(t *testing.T) {
		pos := positions.NewNop()

		handler := loki.NewCollectingHandler()
		defer handler.Stop()

		registry := prometheus.NewRegistry()
		relabelCfg := `
- regex: 'job'
  action: 'labeldrop'`

		j := newFakeJournal()
		tailer := newTailerWithJournal(tailerOptions{
			logger:  slog.New(slog.DiscardHandler),
			metrics: newMetrics(registry),
			id:      "test",
			fanout:  loki.NewFanout([]loki.LogsReceiver{handler.Receiver()}),
			pos:     pos,
			rcs:     parseRelabelRules(t, relabelCfg),
		}, j)
		tailer.Start()
		defer tailer.Stop()

		// No labels but correct message
		for range 10 {
			j.Write(map[string]string{
				"MESSAGE":   "ping",
				"CODE_FILE": "journaltarget_test.go",
			})
		}

		// No labels and no message
		for range 10 {
			j.Write(map[string]string{
				"CODE_FILE": "journaltarget_test.go",
			})
		}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			expectedMetrics := `# HELP loki_source_journal_target_lines_total Total number of successful journal lines read
	# TYPE loki_source_journal_target_lines_total counter
	loki_source_journal_target_lines_total 0
	# HELP loki_source_journal_target_parsing_errors_total Total number of parsing errors while reading journal messages
	# TYPE loki_source_journal_target_parsing_errors_total counter
	loki_source_journal_target_parsing_errors_total{error="empty_labels"} 10
	loki_source_journal_target_parsing_errors_total{error="no_message"} 10
	`
			require.NoError(c, testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics)))
			require.Len(c, handler.Received(), 0)
		}, 3*time.Second, 100*time.Millisecond)
	})

	t.Run("json messages", func(t *testing.T) {
		pos := positions.NewNop()

		handler := loki.NewCollectingHandler()
		defer handler.Stop()

		registry := prometheus.NewRegistry()

		// Keep only entries from this code file, and copy the code file into a
		// real label so the entry isn't dropped for having no labels.
		relabelCfg := `
- source_labels: ['__journal_code_file']
  regex: 'journaltarget_test\.go'
  action: 'keep'
- source_labels: ['__journal_code_file']
  target_label: 'code_file'`

		j := newFakeJournal()

		const numMessages = 10
		for range numMessages {
			j.Write(map[string]string{
				"MESSAGE":     "ping",
				"CODE_FILE":   "journaltarget_test.go",
				"OTHER_FIELD": "foobar",
			})
		}

		tailer := newTailerWithJournal(tailerOptions{
			logger:  slog.New(slog.DiscardHandler),
			metrics: newMetrics(registry),
			id:      "test",
			fanout:  loki.NewFanout([]loki.LogsReceiver{handler.Receiver()}),
			pos:     pos,
			rcs:     parseRelabelRules(t, relabelCfg),
			asJSON:  true,
		}, j)
		tailer.Start()
		defer tailer.Stop()

		// asJSON marshals every field of the entry into the line.
		expectMsg := `{"CODE_FILE":"journaltarget_test.go","MESSAGE":"ping","OTHER_FIELD":"foobar"}`

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			received := handler.Received()
			require.Len(c, received, numMessages)
			for i := range received {
				require.Equal(c, expectMsg, received[i].Line)
			}
		}, 3*time.Second, 100*time.Millisecond)
	})
}

func parseRelabelRules(t *testing.T, s string) []*relabel.Config {
	var relabels []*relabel.Config
	err := yaml.Unmarshal([]byte(s), &relabels)
	require.NoError(t, err)
	for i := range relabels {
		relabels[i].NameValidationScheme = model.LegacyValidation
	}
	return relabels
}

var _ journal = (*fakeJournal)(nil)

func newFakeJournal() *fakeJournal {
	return &fakeJournal{
		written: make(chan struct{}, 1),
	}
}

type fakeJournal struct {
	written chan struct{}

	mut     sync.Mutex
	readpos int
	entries [][]sdjournal.Field
	cursors []string
}

// Next implements journal.
func (f *fakeJournal) Next() ([]sdjournal.Field, string, error) {
	f.mut.Lock()
	defer f.mut.Unlock()
	if f.readpos > len(f.entries)-1 {
		return nil, "", sdjournal.ErrNoData
	}

	entries, cursor := f.entries[f.readpos], f.cursors[f.readpos]
	f.readpos += 1
	return entries, cursor, nil
}

// Realtime implements journal.
func (f *fakeJournal) Realtime() (time.Time, error) {
	return time.Time{}, nil
}

func (f *fakeJournal) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.written:
		return nil
	}
}

func (f *fakeJournal) Close() {}

func (f *fakeJournal) Write(fields map[string]string) {
	f.mut.Lock()
	defer f.mut.Unlock()

	var entry []sdjournal.Field

	for k, v := range fields {
		entry = append(entry, sdjournal.Field{
			Name:  k,
			Value: v,
		})
	}

	f.entries = append(f.entries, entry)
	f.cursors = append(f.cursors, strconv.Itoa(len(f.entries)))
	select {
	case f.written <- struct{}{}:
	default:
	}
}
