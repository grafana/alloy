//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/component/loki/source/journal/internal/sdjournal"
)

type tailerOptions struct {
	logger  *slog.Logger
	metrics *metrics
	fanout  *loki.Fanout

	path string
	id   string
	pos  positions.Positions

	matches string
	maxAge  time.Duration
	rcs     []*relabel.Config
	labels  map[string]string
	asJSON  bool
}

// Bounds of the wait before the journal is reopened after a read error.
const (
	minReopenBackoff = time.Second
	maxReopenBackoff = 30 * time.Second
)

// openFunc opens a journal positioned for the first Next: after cursor (with
// sdjournal's fallbacks for an empty, unreadable or too old cursor), or, with
// fromNow, at the current end of the journal.
type openFunc func(cursor string, fromNow bool) (journal, error)

func newTailer(opts tailerOptions) (*tailer, error) {
	open := func(cursor string, fromNow bool) (journal, error) {
		o := sdjournal.Options{
			Path:    opts.path,
			Cursor:  cursor,
			MaxAge:  opts.maxAge,
			Matches: strings.Fields(opts.matches),
		}
		if fromNow {
			o.Since = time.Now()
		}
		j, err := sdjournal.New(o)
		if err != nil {
			return nil, err
		}
		return j, nil
	}

	journal, err := open(opts.pos.GetString(positions.CursorKey(opts.id), ""), false)
	if err != nil {
		return nil, err
	}

	return newTailerWithJournal(opts, journal, open), nil
}

func newTailerWithJournal(opts tailerOptions, journal journal, open openFunc) *tailer {
	labelMap := make(map[string]string, len(opts.labels)+1)
	maps.Copy(labelMap, opts.labels)
	if labelMap["job"] == "" {
		labelMap["job"] = opts.id
	}
	lbls := labels.FromMap(labelMap)

	ctx, cancel := context.WithCancel(context.Background())

	return &tailer{
		journal: journal,
		open:    open,
		after:   time.After,
		backoff: minReopenBackoff,
		fanout:  opts.fanout,
		logger:  opts.logger,
		metrics: opts.metrics,

		key: positions.CursorKey(opts.id),
		pos: opts.pos,

		rcs:    opts.rcs,
		labels: lbls,
		br:     labels.NewBuilder(lbls),
		asJson: opts.asJSON,

		ctx:    ctx,
		cancel: cancel,
	}
}

type journal interface {
	Next() ([]sdjournal.Field, string, error)
	Realtime() (time.Time, error)
	Wait(ctx context.Context) error
	Close()
}

type tailer struct {
	journal journal
	open    openFunc
	after   func(time.Duration) <-chan time.Time

	// Read error recovery, owned by the reading goroutine: the wait before the
	// next reopen, whether the current handle was reopened and has not yielded
	// an entry yet, whether the skip to the end of the journal was logged, and
	// the receive time of the last entry sent.
	backoff  time.Duration
	reopened bool
	skipped  bool
	lastSent time.Time

	logger  *slog.Logger
	metrics *metrics
	fanout  *loki.Fanout

	rcs    []*relabel.Config
	labels labels.Labels
	br     *labels.Builder

	asJson bool

	key string
	pos positions.Positions

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (t *tailer) Start() {
	t.wg.Go(func() {
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
			}

			fields, cursor, err := t.journal.Next()
			if errors.Is(err, sdjournal.ErrNoData) {
				if err := t.journal.Wait(t.ctx); err != nil && !errors.Is(err, context.Canceled) {
					if !t.reopen("failed waiting for journal entry, reopening the journal", err) {
						return
					}
				}
				continue
			}

			if err != nil {
				if !t.reopen("failed to read journal entry, reopening the journal", err) {
					return
				}
				continue
			}
			t.backoff, t.reopened, t.skipped = minReopenBackoff, false, false

			t.br.Reset(t.labels)

			var line string
			if t.asJson {
				m := make(map[string]string, len(fields))
				setLabels(t.br, fields, func(f sdjournal.Field) {
					m[f.Name] = f.Value
				})

				json := jsoniter.ConfigCompatibleWithStandardLibrary
				bb, err := json.Marshal(m)
				if err != nil {
					t.logger.Error("could not marshal journal fields as JSON", "err", err)
					t.pos.PutString(t.key, "", cursor)
					continue
				}
				line = string(bb)
			} else {
				setLabels(t.br, fields, func(f sdjournal.Field) {
					if f.Name == sdjournal.FieldMessage {
						line = f.Value
					}
				})
			}

			if line == "" {
				t.logger.Debug("received journal entry without MESSAGE field, skipping")
				t.metrics.journalErrors.WithLabelValues(noMessageError).Inc()
				t.pos.PutString(t.key, "", cursor)
				continue
			}

			if !relabel.ProcessBuilder(t.br, t.rcs...) {
				t.logger.Debug("journal entry dropped by relabel rules")
				t.metrics.journalErrors.WithLabelValues(emptyLabelsError).Inc()
				t.pos.PutString(t.key, "", cursor)
				continue
			}

			lset := make(model.LabelSet)
			t.br.Range(func(l labels.Label) {
				if strings.HasPrefix(l.Name, "__") {
					return
				}
				lset[model.LabelName(l.Name)] = model.LabelValue(l.Value)
			})

			if len(lset) == 0 {
				t.logger.Debug("received journal entry without labels")
				t.metrics.journalErrors.WithLabelValues(emptyLabelsError).Inc()
				t.pos.PutString(t.key, "", cursor)
				continue
			}

			ts, err := t.journal.Realtime()
			if err != nil {
				t.logger.Warn("failed to get journal entry time defaulting to now", "err", err)
				ts = time.Now()
			}

			if err := t.fanout.Send(t.ctx, loki.NewEntry(lset, push.Entry{
				Timestamp: ts,
				Line:      line,
			})); err != nil {
				t.logger.Debug("could not forward entry", "err", err)
				continue
			}

			t.metrics.journalLines.Inc()
			t.pos.PutString(t.key, "", cursor)
			t.lastSent = ts
		}
	})
}

// reopen replaces the journal handle after a read error. A failed handle is
// never read again: libsystemd can leave it on an entry it cannot read, failing
// every later call. The new handle resumes after the last stored cursor, as a
// restart would. If a reopened handle fails before yielding an entry, the next
// one starts at the end of the journal, skipping the unreadable span, which is
// logged once. Attempts back off from minReopenBackoff to maxReopenBackoff; a
// successful read resets the backoff. reopen returns false when stopping.
func (t *tailer) reopen(msg string, err error) bool {
	t.logger.Warn(msg, "err", err, "backoff", t.backoff)
	fromNow := t.reopened
	t.journal.Close()
	t.journal = nil

	for {
		select {
		case <-t.ctx.Done():
			return false
		case <-t.after(t.backoff):
		}
		t.backoff = min(2*t.backoff, maxReopenBackoff)

		cursor := t.pos.GetString(t.key, "")
		if fromNow && !t.skipped {
			t.skipped = true
			args := []any{"after_cursor", cursor}
			if !t.lastSent.IsZero() {
				args = append(args, "after_time", t.lastSent)
			}
			t.logger.Warn("journal unreadable after the last read entry, skipping to the end of the journal", args...)
		}

		j, err := t.open(cursor, fromNow)
		if err != nil {
			t.logger.Warn("failed to reopen journal", "err", err, "backoff", t.backoff)
			fromNow = true
			continue
		}
		t.journal = j
		t.reopened = true
		return true
	}
}

func (t *tailer) Stop() {
	t.cancel()
	t.wg.Wait()
	if t.journal != nil {
		t.journal.Close()
	}
}

const labelNamePrefix = "__journal_"

func setLabels(br *labels.Builder, fields []sdjournal.Field, visit func(f sdjournal.Field)) {
	for _, f := range fields {
		if f.Name == sdjournal.FieldPriority {
			br.Set(labelNamePrefix+"priority_keyword", makeJournalPriority(f.Value))
		}
		br.Set(labelNamePrefix+strings.ToLower(f.Name), f.Value)
		visit(f)
	}
}

func makeJournalPriority(priority string) string {
	switch priority {
	case "0":
		return "emerg"
	case "1":
		return "alert"
	case "2":
		return "crit"
	case "3":
		return "error"
	case "4":
		return "warning"
	case "5":
		return "notice"
	case "6":
		return "info"
	case "7":
		return "debug"
	}
	return priority
}
