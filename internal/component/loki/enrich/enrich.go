// Package enrich provides the loki.enrich component.
package enrich

import (
	"context"
	"sync"

	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/loki/pkg/push"
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

	// Which label from targets to use for matching (e.g. "hostname", "ip")
	TargetMatchLabel string `alloy:"target_match_label,attr"`

	// Which label from logs to match against (e.g. "hostname", "ip")
	// If not specified, TargetMatchLabel will be used
	LogsMatchLabel string `alloy:"logs_match_label,attr,optional"`

	// List of labels to copy from discovered targets to logs. If empty, all labels will be copied.
	LabelsToCopy []string `alloy:"labels_to_copy,attr,optional"`

	// Where to forward logs after enrichment
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr,optional"`
}

type Component struct {
	opts    component.Options
	args    Arguments
	exports Exports

	mut          sync.RWMutex
	receiver     loki.LogsReceiver
	targetsCache map[string]model.LabelSet
	cacheMutex   sync.RWMutex
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:         opts,
		args:         args,
		targetsCache: make(map[string]model.LabelSet),
		receiver:     loki.NewLogsReceiver(),
	}

	// Initialize the cache with provided targets
	c.refreshCacheFromTargets(args.Targets)

	// Create and immediately export the receiver
	c.exports.Receiver = c.receiver
	opts.OnStateChange(c.exports)

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			if err := c.processLog(&entry.Entry, entry.Labels); err != nil {
				level.Error(c.opts.Logger).Log("msg", "failed to process log", "err", err)
			}
		}
	}
}

func (c *Component) refreshCacheFromTargets(targets []discovery.Target) {
	newCache := make(map[string]model.LabelSet)

	for _, target := range targets {
		labelSet := make(model.LabelSet)
		// Copy both own and group labels
		target.ForEachLabel(func(k, v string) bool {
			labelSet[model.LabelName(k)] = model.LabelValue(v)
			return true
		})
		if matchValue := string(labelSet[model.LabelName(c.args.TargetMatchLabel)]); matchValue != "" {
			newCache[matchValue] = labelSet
		}
	}

	c.cacheMutex.Lock()
	c.targetsCache = newCache
	c.cacheMutex.Unlock()
}

func (c *Component) processLog(entry *push.Entry, labels model.LabelSet) error {
	// Determine which label to use for matching
	matchLabel := c.args.LogsMatchLabel
	if matchLabel == "" {
		matchLabel = c.args.TargetMatchLabel
	}

	// Get the source value to match against discovered targets
	sourceValue := string(labels[model.LabelName(matchLabel)])
	if sourceValue == "" {
		// No match label, forward as-is
		return c.forwardLog(entry, labels)
	}

	// Look up matching target
	c.cacheMutex.RLock()
	targetLabels, found := c.targetsCache[sourceValue]
	c.cacheMutex.RUnlock()

	if !found {
		// No matching target, forward as-is
		return c.forwardLog(entry, labels)
	}

	// Copy labels from target to log labels
	newLabels := labels.Clone()
	if len(c.args.LabelsToCopy) == 0 {
		// If no specific labels are requested, copy all labels
		for k, v := range targetLabels {
			newLabels[k] = v
		}
	} else {
		// Copy only requested labels
		for _, label := range c.args.LabelsToCopy {
			if value := targetLabels[model.LabelName(label)]; value != "" {
				newLabels[model.LabelName(label)] = value
			}
		}
	}

	return c.forwardLog(entry, newLabels)
}

func (c *Component) forwardLog(entry *push.Entry, labels model.LabelSet) error {
	c.mut.RLock()
	fanout := c.args.ForwardTo
	c.mut.RUnlock()

	for _, receiver := range fanout {
		receiver.Chan() <- loki.Entry{
			Labels: labels,
			Entry:  *entry,
		}
	}
	return nil
}

func (c *Component) Name() string {
	return "loki.enrich"
}

func (c *Component) Ready() bool {
	return true
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = newArgs

	// Update the targets cache with new targets
	c.refreshCacheFromTargets(newArgs.Targets)

	return nil
}

func (c *Component) Exports() component.Exports {
	return &c.exports
}
