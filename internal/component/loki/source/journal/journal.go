//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/loki/v3/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/journal/internal/target"
	"github.com/grafana/alloy/internal/featuregate"

	"github.com/grafana/alloy/internal/component"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.journal",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

var _ component.Component = (*Component)(nil)

// Component represents reading from a journal
type Component struct {
	mut       sync.RWMutex
	t         *target.JournalTarget
	metrics   *target.Metrics
	o         component.Options
	handler   chan loki.Entry
	positions positions.Positions
	receivers []loki.LogsReceiver
}

// New creates a new  component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil {
		return nil, err
	}

	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:        10 * time.Second,
		PositionsFile:     filepath.Join(o.DataPath, "positions.yml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	if err != nil {
		return nil, err
	}

	c := &Component{
		metrics:   target.NewMetrics(o.Registerer),
		o:         o,
		handler:   make(chan loki.Entry),
		positions: positionsFile,
		receivers: args.Receivers,
	}
	err = c.Update(args)
	return c, err
}

// Run starts the component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.RLock()
		if c.t != nil {
			c.t.Stop()
		}
		c.mut.RUnlock()

	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler:
			c.mut.RLock()
			lokiEntry := loki.Entry{
				Labels: entry.Labels,
				Entry:  entry.Entry,
			}
			for _, r := range c.receivers {
				r.Chan() <- lokiEntry
			}
			c.mut.RUnlock()
		}
	}
}

// Update updates the fields of the component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.t != nil {
		err := c.t.Stop()
		if err != nil {
			return err
		}
	}
	rcs := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	entryHandler := loki.NewEntryHandler(c.handler, func() {})

	newTarget, err := target.NewJournalTarget(c.metrics, c.o.Logger, entryHandler, c.positions, c.o.ID, rcs, convertArgs(c.o.ID, newArgs))
	if err != nil {
		return err
	}
	c.t = newTarget
	return nil
}

func convertArgs(job string, a Arguments) *scrapeconfig.JournalTargetConfig {
	labels := model.LabelSet{
		model.LabelName("job"): model.LabelValue(job),
	}

	for k, v := range a.Labels {
		labels[model.LabelName(k)] = model.LabelValue(v)
	}

	return &scrapeconfig.JournalTargetConfig{
		MaxAge:  a.MaxAge.String(),
		JSON:    a.FormatAsJson,
		Labels:  labels,
		Path:    a.Path,
		Matches: a.Matches,
	}
}
