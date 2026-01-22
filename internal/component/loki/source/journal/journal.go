//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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
	opts           component.Options
	metrics        *metrics
	recv           loki.LogsReceiver
	positions      positions.Positions
	targetsUpdated chan struct{}

	fanout *loki.Fanout

	mut       sync.RWMutex
	tailer    *tailer
	args      Arguments
	healthErr error
}

// New creates a new  component.
func New(o component.Options, args Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil {
		return nil, err
	}

	positionFile := filepath.Join(o.DataPath, "positions.yml")
	if args.LegacyPosition != nil {
		positions.ConvertLegacyPositionsFileJournal(args.LegacyPosition.File, args.LegacyPosition.Name, positionFile, o.ID, o.Logger)
	}

	positionsFile, err := positions.New(o.Logger, positions.Config{
		SyncPeriod:        10 * time.Second,
		PositionsFile:     positionFile,
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})
	if err != nil {
		return nil, err
	}

	c := &Component{
		metrics:        newMetrics(o.Registerer),
		opts:           o,
		recv:           loki.NewLogsReceiver(),
		positions:      positionsFile,
		fanout:         loki.NewFanout(args.ForwardTo),
		targetsUpdated: make(chan struct{}, 1),
		args:           args,
	}
	err = c.Update(args)
	return c, err
}

// Run starts the component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", "loki.source.journal component shutting down")
		// We need to stop posFile first so we don't record entries we are draining
		c.positions.Stop()

		source.Drain(c.recv, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			if c.tailer != nil {
				if err := c.tailer.Stop(); err != nil {
					level.Warn(c.opts.Logger).Log("msg", "error stopping journal tailer", "err", err)
				}
			}
		})
	}()

	var wg sync.WaitGroup
	wg.Go(func() { source.Consume(ctx, c.recv, c.fanout) })
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.targetsUpdated:
				c.reloadTailer()
			}
		}

	})

	wg.Wait()
	return nil
}

// Update updates the fields of the component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.mut.Lock()
	defer c.mut.Unlock()

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.args = newArgs
	select {
	case c.targetsUpdated <- struct{}{}:
	default: // Update notification already sent
	}
	return nil
}

// CurrentHealth implements component.HealthComponent. It returns an unhealthy
// status if the server has terminated.
func (c *Component) CurrentHealth() component.Health {
	c.mut.RLock()
	defer c.mut.RUnlock()
	if c.healthErr == nil {
		return component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "journal tailer is running",
			UpdateTime: time.Now(),
		}
	}
	return component.Health{
		Health:     component.HealthTypeUnhealthy,
		Message:    c.healthErr.Error(),
		UpdateTime: time.Now(),
	}
}

func (c *Component) reloadTailer() {
	// Grab current state
	c.mut.RLock()
	var tailerToStop *tailer
	if c.tailer != nil {
		tailerToStop = c.tailer
	}
	rcs := alloy_relabel.ComponentToPromRelabelConfigs(c.args.RelabelRules)
	c.mut.RUnlock()

	// Stop existing tailer
	if tailerToStop != nil {
		err := tailerToStop.Stop()
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error stopping journal tailer", "err", err)
		}
	}

	// Create new tailer
	c.mut.Lock()
	defer c.mut.Unlock()
	c.tailer = nil

	tailer, err := newTailer(c.metrics, c.opts.Logger, c.recv, c.positions, c.opts.ID, rcs, convertArgs(c.opts.ID, c.args))
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "error creating journal tailer", "err", err, "path", c.args.Path)
		c.healthErr = fmt.Errorf("error creating journal tailer: %w", err)
	} else {
		c.tailer = tailer
		c.healthErr = nil
	}
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
