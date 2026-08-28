// Package enrich provides the loki.enrich component.
package enrich

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
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

	// Multi-label matching: a map of target_label -> log_label.
	// Takes precedence over target_match_label / logs_match_label.
	TargetToLogMatch map[string]string `alloy:"target_to_log_match,attr,optional"`

	// Legacy: which label from targets to use for matching (e.g. "hostname", "ip").
	TargetMatchLabel string `alloy:"target_match_label,attr,optional"`

	// Legacy: which label from logs to match against (e.g. "hostname", "ip").
	// If not specified, TargetMatchLabel will be used.
	LogsMatchLabel string `alloy:"logs_match_label,attr,optional"`

	// List of labels to copy from discovered targets to logs. If empty, all labels will be copied.
	LabelsToCopy []string `alloy:"labels_to_copy,attr,optional"`

	// Where to forward logs after enrichment
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

// Validate implements syntax.Validator.
func (a Arguments) Validate() error {
	hasLegacy := a.TargetMatchLabel != "" || a.LogsMatchLabel != ""
	hasNew := len(a.TargetToLogMatch) > 0

	if !hasLegacy && !hasNew {
		return fmt.Errorf("at least one match mechanism must be specified: set target_match_label or target_to_log_match")
	}
	// target_to_log_match takes precedence when set; legacy fields are ignored.
	if hasLegacy && !hasNew && a.TargetMatchLabel == "" {
		return fmt.Errorf("target_match_label must be set when using legacy match fields")
	}
	return nil
}

type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr,optional"`
}

var sep = []byte{0xff} // separator to prevent hash collisions across value boundaries

// hashValuesFromLabelSet hashes the values of the given label names (in order)
// from a model.LabelSet. Returns (0, false) if names is empty or any label is
// missing or empty.
func hashValuesFromLabelSet(ls model.LabelSet, names []string) (uint64, bool) {
	if len(names) == 0 {
		return 0, false
	}
	h := xxhash.New()
	for _, name := range names {
		v := string(ls[model.LabelName(name)])
		if v == "" {
			return 0, false
		}
		_, _ = h.WriteString(v)
		_, _ = h.Write(sep)
	}
	return h.Sum64(), true
}

// matchCache holds the hash-based lookup for a match strategy.
type matchCache struct {
	sortedLogLabels []string                  // log label names to hash, sorted by corresponding target label name
	cache           map[uint64]model.LabelSet // hash of values -> target label set
	labelsToCopy    []string                  // snapshot of which target labels to copy (empty means copy all)
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

			batch.FilterMapStreams(func(stream *loki.Stream) (keep bool) {
				stream.Labels = c.process(stream.Labels, false)
				return true
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
	if c.mc == nil {
		return lset
	}

	h, ok := hashValuesFromLabelSet(lset, c.mc.sortedLogLabels)
	if !ok {
		return lset
	}
	targetLabels, found := c.mc.cache[h]
	if !found {
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

// sortStrategyMap converts a target_label->log_label map into sorted parallel
// slices for deterministic hashing.
func sortStrategyMap(m map[string]string) (targetLabels, logLabels []string) {
	targetLabels = make([]string, 0, len(m))
	for k := range m {
		targetLabels = append(targetLabels, k)
	}
	sort.Strings(targetLabels)

	logLabels = make([]string, 0, len(targetLabels))
	for _, k := range targetLabels {
		logLabels = append(logLabels, m[k])
	}
	return targetLabels, logLabels
}

func (c *Component) refreshCacheFromTargets(args Arguments) {
	strategyMap := args.TargetToLogMatch
	if len(strategyMap) == 0 && args.TargetMatchLabel != "" {
		logsLabel := args.LogsMatchLabel
		if logsLabel == "" {
			logsLabel = args.TargetMatchLabel
		}
		strategyMap = map[string]string{args.TargetMatchLabel: logsLabel}
	}

	sortedTargetLabels, sortedLogLabels := sortStrategyMap(strategyMap)
	cache := make(map[uint64]model.LabelSet)
	for _, target := range args.Targets {
		lset := target.LabelSet()
		h, ok := hashValuesFromLabelSet(lset, sortedTargetLabels)
		if !ok {
			continue
		}
		cache[h] = lset
	}
	c.mc = &matchCache{
		sortedLogLabels: sortedLogLabels,
		cache:           cache,
		labelsToCopy:    args.LabelsToCopy,
	}
}
