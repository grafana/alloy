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

func newTailer(opts tailerOptions) (*tailer, error) {
	key := positions.CursorKey(opts.id)
	cursor := opts.pos.GetString(key, "")

	journal, err := sdjournal.New(sdjournal.Options{
		Path:    opts.path,
		Cursor:  cursor,
		MaxAge:  opts.maxAge,
		Matches: strings.Fields(opts.matches),
	})
	if err != nil {
		return nil, err
	}

	return newTailerWithJournal(opts, journal), nil
}

func newTailerWithJournal(opts tailerOptions, journal journal) *tailer {
	labelMap := make(map[string]string, len(opts.labels)+1)
	maps.Copy(labelMap, opts.labels)
	labelMap["job"] = opts.id
	lbls := labels.FromMap(labelMap)

	ctx, cancel := context.WithCancel(context.Background())

	return &tailer{
		journal: journal,
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
					t.logger.Error("failed waiting for journal entry", "err", err)
					return
				}
				continue
			}

			if err != nil {
				t.logger.Error("failed to read journal entry", "err", err)
				select {
				case <-t.ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}

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
		}
	})
}

func (t *tailer) Stop() {
	t.cancel()
	t.wg.Wait()
	t.journal.Close()
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
