// Package enrich provides the loki.enrich component.
package enrich

import (
	"context"
	"sync"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	commonenrich "github.com/grafana/alloy/internal/component/common/enrich"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.enrich",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the loki.enrich component.
type Arguments struct {
	// The targets to use for enrichment
	Targets []discovery.Target `alloy:"targets,attr"`

	// Map of target label -> log label. All pairs must match the same target.
	TargetToLogMatch map[string]string `alloy:"target_to_log_match,attr"`

	// List of labels to copy from discovered targets to logs. If empty, all labels will be copied.
	LabelsToCopy []string `alloy:"labels_to_copy,attr,optional"`

	// Where to forward logs after enrichment
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

// Validate implements syntax.Validator.
func (a Arguments) Validate() error {
	return commonenrich.ValidateMatch("target_to_log_match", a.TargetToLogMatch)
}

type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr,optional"`
}

// matchCache holds the hash-based lookup for a match strategy.
type matchCache struct {
	matcher      *commonenrich.Matcher
	labelsToCopy []string // snapshot of which target labels to copy (empty means copy all)
}

type Component struct {
	opts component.Options

	receiver    loki.LogsReceiver
	fanout      *loki.Fanout
	interceptor *loki.InterceptorConsumer

	mut     sync.RWMutex
	stopped bool
	mc      *matchCache
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:     opts,
		receiver: loki.NewLogsReceiver(loki.WithComponentID(opts.ID)),
		fanout:   loki.NewFanout(args.ForwardTo),
	}

	c.interceptor = loki.NewInterceptorConsumer(
		opts.ID,
		loki.NewNopConsumer(),
		func(ctx context.Context, batch loki.Batch) (loki.Batch, error) {
			c.mut.RLock()
			defer c.mut.RUnlock()
			if c.stopped {
				return loki.Batch{}, loki.ErrConsumerStopped
			}

			batch.FilterMapStreams(func(stream loki.Stream) (loki.Stream, bool) {
				stream.Labels = c.process(stream.Labels, false)
				return stream, true
			})

			return batch, nil
		},
	)

	opts.OnStateChange(Exports{Receiver: c.receiver})

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.stopped = true
	}()

	loki.ConsumeAndProcess(ctx, c.receiver, c.fanout, func(e loki.Entry) (loki.Entry, bool) {
		c.mut.RLock()
		defer c.mut.RUnlock()

		e.Labels = c.process(e.Labels, true)
		return e, true
	})
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	// Update the targets cache with new targets
	c.refreshCacheFromTargets(newArgs)

	return nil
}

// process returns lset enriched with labels from a matching target. Set
// needsClone when the caller does not own lset.
func (c *Component) process(lset model.LabelSet, needsClone bool) model.LabelSet {
	// Skip the hash work when there is nothing to match.
	if c.mc == nil || c.mc.matcher.Len() == 0 {
		return lset
	}

	targetLabels := c.mc.matcher.Match(func(name string) string {
		return string(lset[model.LabelName(name)])
	})
	if targetLabels == nil {
		return lset
	}

	if needsClone {
		lset = lset.Clone()
	}

	if len(c.mc.labelsToCopy) == 0 {
		// If no specific labels are requested, copy all labels
		for k, v := range targetLabels {
			lset[k] = v
		}
	} else {
		// Copy only requested labels
		for _, label := range c.mc.labelsToCopy {
			if value := targetLabels[model.LabelName(label)]; value != "" {
				lset[model.LabelName(label)] = value
			}
		}
	}
	return lset
}

func (c *Component) refreshCacheFromTargets(args Arguments) {
	c.mc = &matchCache{
		matcher:      commonenrich.NewMatcher(args.Targets, args.TargetToLogMatch),
		labelsToCopy: args.LabelsToCopy,
	}
}
