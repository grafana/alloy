//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
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
		}, j, nil)
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
		}, j, nil)
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

func TestTailerRecovery(t *testing.T) {
	errBadMessage := errors.New("sd_journal_next failed: bad message")

	// run starts a tailer on first; opens are served from next in order (a nil
	// journal is an open error) and every backoff wait fires at once.
	run := func(t *testing.T, first *fakeJournal, next ...*fakeJournal) *recoveryRun {
		r := &recoveryRun{handler: loki.NewCollectingHandler()}
		t.Cleanup(r.handler.Stop)
		pos, err := positions.New(slog.New(slog.DiscardHandler), positions.Config{
			Path: filepath.Join(t.TempDir(), "positions.yml"),
		})
		require.NoError(t, err)
		t.Cleanup(pos.Stop)

		r.tailer = newTailerWithJournal(tailerOptions{
			logger:  slog.New(slog.NewTextHandler(&r.logs, nil)),
			metrics: newMetrics(prometheus.NewRegistry()),
			id:      "test",
			fanout:  loki.NewFanout([]loki.LogsReceiver{r.handler.Receiver()}),
			pos:     pos,
		}, first, func(cursor string, fromNow bool) (journal, error) {
			r.mut.Lock()
			defer r.mut.Unlock()
			r.opens = append(r.opens, openCall{cursor, fromNow})
			if len(next) == 0 {
				return newFakeJournal(), nil // healthy and empty: the tailer waits
			}
			j := next[0]
			next = next[1:]
			if j == nil {
				return nil, errors.New("sd_journal_seek_cursor failed: bad message")
			}
			return j, nil
		})
		r.tailer.after = func(d time.Duration) <-chan time.Time {
			r.mut.Lock()
			defer r.mut.Unlock()
			r.waits = append(r.waits, d)
			c := make(chan time.Time, 1)
			c <- time.Time{}
			return c
		}
		r.tailer.Start()
		t.Cleanup(r.tailer.Stop)
		return r
	}

	t.Run("a failed handle is reopened after the last stored cursor", func(t *testing.T) {
		first := newFakeJournal("a", "a1", "a2").failing(errBadMessage)
		r := run(t, first, newFakeJournal("b", "b1"))

		r.requireLines(t, "a1", "a2", "b1")
		r.requireOpens(t, openCall{"a2", false})
		r.requireWaits(t, time.Second)
		require.Equal(t, 1, first.closes())
		require.NotContains(t, r.logs.String(), "skipping")
	})

	t.Run("a reopened handle that fails before an entry skips to the end once", func(t *testing.T) {
		first := newFakeJournal("a", "a1").failing(errBadMessage)
		second := newFakeJournal("b").failing(errBadMessage)
		r := run(t, first, second, newFakeJournal("c", "c1"))

		r.requireLines(t, "a1", "c1")
		r.requireOpens(t, openCall{"a1", false}, openCall{"a1", true})
		r.requireWaits(t, time.Second, 2*time.Second)
		require.Equal(t, 1, second.closes())
		require.Equal(t, 1, strings.Count(r.logs.String(), "skipping to the end of the journal"))
		require.Contains(t, r.logs.String(), "after_cursor=a1")
	})

	t.Run("a persistent failure backs off to the cap and logs the skip once", func(t *testing.T) {
		failing := func() *fakeJournal { return newFakeJournal("x").failing(errBadMessage) }
		r := run(t, failing(), failing(), failing(), nil, failing(), failing(), failing(), failing())

		r.requireOpens(t, openCall{"", false}, openCall{"", true}, openCall{"", true}, openCall{"", true},
			openCall{"", true}, openCall{"", true}, openCall{"", true}, openCall{"", true})
		r.requireWaits(t, time.Second, 2*time.Second, 4*time.Second, 8*time.Second, 16*time.Second,
			30*time.Second, 30*time.Second, 30*time.Second)
		require.Equal(t, 1, strings.Count(r.logs.String(), "skipping to the end of the journal"))
		require.Contains(t, r.logs.String(), "failed to reopen journal")
	})

	t.Run("a successful read resets the backoff", func(t *testing.T) {
		r := run(t,
			newFakeJournal("a", "a1").failing(errBadMessage),
			newFakeJournal("b", "b1").failing(errBadMessage),
			newFakeJournal("c", "c1"),
		)

		r.requireLines(t, "a1", "b1", "c1")
		r.requireOpens(t, openCall{"a1", false}, openCall{"b1", false})
		r.requireWaits(t, time.Second, time.Second)
		require.NotContains(t, r.logs.String(), "skipping")
	})

	t.Run("a failed wait reopens the journal", func(t *testing.T) {
		first := newFakeJournal("a", "a1")
		first.waitErr = errBadMessage
		r := run(t, first, newFakeJournal("b", "b1"))

		r.requireLines(t, "a1", "b1")
		r.requireOpens(t, openCall{"a1", false})
		require.Equal(t, 1, first.closes())
	})

	t.Run("stopping during the backoff closes the failed handle once", func(t *testing.T) {
		first := newFakeJournal("a").failing(errBadMessage)
		tailer := newTailerWithJournal(tailerOptions{
			logger:  slog.New(slog.DiscardHandler),
			metrics: newMetrics(prometheus.NewRegistry()),
			id:      "test",
			fanout:  loki.NewFanout(nil),
			pos:     positions.NewNop(),
		}, first, func(string, bool) (journal, error) {
			t.Error("reopened after Stop")
			return nil, errors.New("unexpected")
		})
		waiting := make(chan struct{})
		tailer.after = func(time.Duration) <-chan time.Time {
			close(waiting)
			return nil // never fires
		}
		tailer.Start()
		<-waiting
		tailer.Stop()
		require.Equal(t, 1, first.closes())
	})
}

type openCall struct {
	cursor  string
	fromNow bool
}

type recoveryRun struct {
	tailer  *tailer
	handler *loki.CollectingHandler
	logs    syncBuffer

	mut   sync.Mutex
	opens []openCall
	waits []time.Duration
}

func (r *recoveryRun) requireLines(t *testing.T, lines ...string) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var got []string
		for _, e := range r.handler.Received() {
			got = append(got, e.Line)
		}
		require.Equal(c, lines, got)
	}, 3*time.Second, 10*time.Millisecond)
}

func (r *recoveryRun) requireOpens(t *testing.T, opens ...openCall) {
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		r.mut.Lock()
		defer r.mut.Unlock()
		require.Equal(c, opens, r.opens)
	}, 3*time.Second, 10*time.Millisecond)
}

func (r *recoveryRun) requireWaits(t *testing.T, waits ...time.Duration) {
	r.mut.Lock()
	defer r.mut.Unlock()
	require.Equal(t, waits, r.waits)
}

type syncBuffer struct {
	mut sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mut.Lock()
	defer b.mut.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mut.Lock()
	defer b.mut.Unlock()
	return b.buf.String()
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

// newFakeJournal returns a journal holding one entry per message, whose
// cursors are name followed by the entry's number.
func newFakeJournal(nameAndMessages ...string) *fakeJournal {
	f := &fakeJournal{
		written: make(chan struct{}, 1),
	}
	if len(nameAndMessages) > 0 {
		f.name = nameAndMessages[0]
		for _, m := range nameAndMessages[1:] {
			f.Write(map[string]string{"MESSAGE": m})
		}
	}
	return f
}

type fakeJournal struct {
	name    string
	written chan struct{}

	mut     sync.Mutex
	readpos int
	entries [][]sdjournal.Field
	cursors []string
	nextErr error // returned by Next once every entry was read
	waitErr error // returned by Wait
	closed  int
}

// failing makes Next fail with err once every entry was read.
func (f *fakeJournal) failing(err error) *fakeJournal {
	f.nextErr = err
	return f
}

func (f *fakeJournal) closes() int {
	f.mut.Lock()
	defer f.mut.Unlock()
	return f.closed
}

// Next implements journal.
func (f *fakeJournal) Next() ([]sdjournal.Field, string, error) {
	f.mut.Lock()
	defer f.mut.Unlock()
	if f.readpos > len(f.entries)-1 {
		if f.nextErr != nil {
			return nil, "", f.nextErr
		}
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
	if f.waitErr != nil {
		return f.waitErr
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.written:
		return nil
	}
}

func (f *fakeJournal) Close() {
	f.mut.Lock()
	defer f.mut.Unlock()
	f.closed++
}

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
	f.cursors = append(f.cursors, f.name+strconv.Itoa(len(f.entries)))
	select {
	case f.written <- struct{}{}:
	default:
	}
}
