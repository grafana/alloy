//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/loki/v3/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/journal/internal/target"
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
	mut            sync.RWMutex
	t              *target.JournalTarget
	metrics        *target.Metrics
	o              component.Options
	handler        chan loki.Entry
	positions      positions.Positions
	targetsUpdated chan struct{}
	args           Arguments
	healthErr      error
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
		metrics:        target.NewMetrics(o.Registerer),
		o:              o,
		handler:        make(chan loki.Entry),
		positions:      positionsFile,
		targetsUpdated: make(chan struct{}, 1),
		args:           args,
	}
	err = c.Update(args)
	return c, err
}

// Run starts the component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		// Start draining routine to prevent potential deadlock if target attempts to send during Stop().
		cancel := c.startDrainingRoutine()
		defer cancel()

		// Stop existing target
		c.mut.RLock()
		defer c.mut.RUnlock()
		if c.t != nil {
			level.Info(c.o.Logger).Log("msg", "loki.source.journal component shutting down, stopping journal target")
			err := c.t.Stop()
			if err != nil {
				level.Warn(c.o.Logger).Log("msg", "error stopping journal target", "err", err)
			}
		}
	}()
	for {
		select {
		case <-c.targetsUpdated:
			c.reloadTargets()
		case <-ctx.Done():
			return nil
		case entry := <-c.handler:
			c.mut.RLock()
			lokiEntry := loki.Entry{
				Labels: entry.Labels,
				Entry:  entry.Entry,
			}
			for _, r := range c.args.Receivers {
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
			Message:    "journal target is running",
			UpdateTime: time.Now(),
		}
	}
	return component.Health{
		Health:     component.HealthTypeUnhealthy,
		Message:    c.healthErr.Error(),
		UpdateTime: time.Now(),
	}
}

func (c *Component) startDrainingRoutine() func() {
	readCtx, cancel := context.WithCancel(context.Background())
	c.mut.RLock()
	defer c.mut.RUnlock()
	receiversCopy := make([]loki.LogsReceiver, len(c.args.Receivers))
	copy(receiversCopy, c.args.Receivers)
	go func() {
		for {
			select {
			case <-readCtx.Done():
				return
			case entry := <-c.handler:
				lokiEntry := loki.Entry{
					Labels: entry.Labels,
					Entry:  entry.Entry,
				}
				for _, r := range receiversCopy {
					r.Chan() <- lokiEntry
				}
			}
		}
	}()
	return cancel
}

func (c *Component) reloadTargets() {
	// Start draining routine to prevent potential deadlock if target attempts to send during Stop().
	cancel := c.startDrainingRoutine()

	// Grab current state
	c.mut.RLock()
	var targetToStop *target.JournalTarget
	if c.t != nil {
		targetToStop = c.t
	}
	rcs := alloy_relabel.ComponentToPromRelabelConfigs(c.args.RelabelRules)
	c.mut.RUnlock()

	// Stop existing target
	if targetToStop != nil {
		err := targetToStop.Stop()
		if err != nil {
			level.Error(c.o.Logger).Log("msg", "error stopping journal target", "err", err)
		}
	}

	// Stop draining routine
	cancel()

	// Create new target
	c.mut.Lock()
	defer c.mut.Unlock()
	c.t = nil
	entryHandler := loki.NewEntryHandler(c.handler, func() {})

	newTarget, err := target.NewJournalTarget(c.metrics, c.o.Logger, entryHandler, c.positions, c.o.ID, rcs, convertArgs(c.o.ID, c.args))
	if err != nil {
		level.Error(c.o.Logger).Log("msg", "error creating journal target", "err", err, "path", c.args.Path)
		c.healthErr = fmt.Errorf("error creating journal target: %w", err)
	} else {
		c.t = newTarget
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
