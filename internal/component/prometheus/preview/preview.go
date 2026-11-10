// Package preview measures a Prometheus pipeline without forwarding its samples.
package preview

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.preview",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments controls measurement and the destinations for generated reports.
type Arguments struct {
	ReportTo       []storage.AppendableV2 `alloy:"report_to,attr"`
	ReportInterval time.Duration          `alloy:"report_interval,attr,optional"`
	Window         time.Duration          `alloy:"window,attr,optional"`
	GroupBy        []string               `alloy:"group_by,attr,optional"`
	MaxGroups      int                    `alloy:"max_groups,attr,optional"`
	MaxSeries      int                    `alloy:"max_series,attr,optional"`
}

var DefaultArguments = Arguments{
	ReportInterval: time.Minute,
	Window:         30 * time.Minute,
	MaxGroups:      100,
	MaxSeries:      100_000,
}

func (a *Arguments) SetToDefault() { *a = DefaultArguments }

var labelName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func (a Arguments) Validate() error {
	if len(a.ReportTo) == 0 {
		return errors.New("report_to must contain at least one receiver")
	}
	for _, receiver := range a.ReportTo {
		if receiver == nil {
			return errors.New("report_to must not contain nil receivers")
		}
	}
	if a.ReportInterval <= 0 {
		return errors.New("report_interval must be positive")
	}
	if a.Window < a.ReportInterval {
		return errors.New("window must be at least report_interval")
	}
	if a.MaxGroups <= 0 || a.MaxSeries <= 0 {
		return errors.New("max_groups and max_series must be positive")
	}
	if len(a.GroupBy) > 16 {
		return errors.New("group_by must contain at most 16 label names")
	}
	seen := make(map[string]bool, len(a.GroupBy))
	for _, name := range a.GroupBy {
		if len(name) > 128 || !labelName.MatchString(name) {
			return fmt.Errorf("invalid group_by label name %q: use at most 128 ASCII letters, digits, or underscores, with no leading digit", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate group_by label name %q", name)
		}
		seen[name] = true
	}
	return nil
}

type Exports struct {
	Receiver storage.AppendableV2 `alloy:"receiver,attr"`
}

type Component struct {
	opts           component.Options
	now            func() time.Time
	updated        chan struct{}
	reportFailures prometheus.Counter

	mu            sync.Mutex
	args          Arguments
	stats         *aggregation
	latestReport  *reportSnapshot
	generation    uint64
	started       time.Time
	windowStarted time.Time
	stopped       bool
}

var (
	_ component.DebugComponent = (*Component)(nil)
	_ storage.Appendable       = (*Component)(nil)
	_ storage.AppendableV2     = (*Component)(nil)
)

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:    opts,
		now:     time.Now,
		updated: make(chan struct{}, 1),
		reportFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "prometheus_preview_report_failures_total",
			Help: "Number of reports that failed to reach one or more receivers.",
		}),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	if opts.Registerer != nil {
		if err := opts.Registerer.Register(c.reportFailures); err != nil {
			return nil, err
		}
	}
	if opts.OnStateChange != nil {
		opts.OnStateChange(Exports{Receiver: c})
	}
	return c, nil
}

func (c *Component) Update(args component.Arguments) error {
	a := args.(Arguments)
	if err := a.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stats == nil || c.args.Window != a.Window || c.args.MaxSeries != a.MaxSeries ||
		c.args.MaxGroups != a.MaxGroups || !slices.Equal(c.args.GroupBy, a.GroupBy) {

		c.stats = newAggregation(a)
		c.latestReport = nil
		c.generation++
		c.started = c.now()
		c.windowStarted = c.started
	}
	intervalChanged := c.args.ReportInterval != a.ReportInterval
	c.args = a
	if intervalChanged {
		select {
		case c.updated <- struct{}{}:
		default:
		}
	}
	return nil
}

func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.stopped = true
		c.stats = nil
	}()

	c.mu.Lock()
	interval := c.args.ReportInterval
	c.mu.Unlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.updated:
			c.mu.Lock()
			interval = c.args.ReportInterval
			c.mu.Unlock()
			ticker.Reset(interval)
		case <-ticker.C:
			if err := c.report(ctx); err != nil {
				c.reportFailures.Inc()
				c.opts.Logger.Error("failed to send preview report", "err", err)
			}
		}
	}
}
