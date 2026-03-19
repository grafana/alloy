// Package enrich provides the loki.enrich component.
package enrich

import (
	"context"
	"sync"

	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
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
	opts     component.Options
	receiver loki.LogsReceiver
	fanout   *loki.Fanout

	mut          sync.RWMutex
	args         Arguments
	targetsCache map[string]model.LabelSet
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:         opts,
		args:         args,
		targetsCache: make(map[string]model.LabelSet),
		receiver:     loki.NewLogsReceiver(loki.WithComponentID(opts.ID)),
		fanout:       loki.NewFanout(args.ForwardTo),
	}

	opts.OnStateChange(Exports{Receiver: c.receiver})

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	loki.ConsumeAndProcess(ctx, c.receiver, c.fanout, func(e loki.Entry) loki.Entry {
		return c.processLog(e)
	})
	return nil
}

func (c *Component) processLog(entry loki.Entry) loki.Entry {
	c.mut.RLock()
	defer c.mut.RUnlock()

	targetMatchLabel := c.args.TargetMatchLabel
	logsMatchLabel := c.args.LogsMatchLabel
	labelsToCopy := append([]string(nil), c.args.LabelsToCopy...)

	// Determine which label to use for matching
	matchLabel := logsMatchLabel
	if matchLabel == "" {
		matchLabel = targetMatchLabel
	}

	// Get the source value to match against discovered targets
	sourceValue := string(entry.Labels[model.LabelName(matchLabel)])
	if sourceValue == "" {
		// No match label, forward as-is
		return entry
	}

	// Look up matching target
	targetLabels, found := c.targetsCache[sourceValue]

	if !found {
		// No matching target, forward as-is
		return entry
	}

	// Copy entry in case it was forwarded to several components.
	newEntry := entry.Clone()
	if len(labelsToCopy) == 0 {
		// If no specific labels are requested, copy all labels
		for k, v := range targetLabels {
			newEntry.Labels[k] = v
		}
	} else {
		// Copy only requested labels
		for _, label := range labelsToCopy {
			if value := targetLabels[model.LabelName(label)]; value != "" {
				newEntry.Labels[model.LabelName(label)] = value
			}
		}
	}

	return newEntry
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.args = newArgs
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	// Update the targets cache with new targets
	c.refreshCacheFromTargets(newArgs.Targets)

	return nil
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

	c.targetsCache = newCache
}
