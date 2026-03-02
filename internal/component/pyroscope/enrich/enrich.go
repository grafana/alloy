// Package enrich provides the pyroscope.enrich component.
package enrich

import (
	"context"
	"sync"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/prometheus/model/labels"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.enrich",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the pyroscope.enrich component.
type Arguments struct {
	// The targets to use for enrichment
	Targets []discovery.Target `alloy:"targets,attr"`

	// Which label from targets to use for matching (e.g. "hostname", "ip")
	TargetMatchLabel string `alloy:"target_match_label,attr"`

	// Which label from profiles to match against (e.g. "hostname", "ip")
	// If not specified, TargetMatchLabel will be used
	ProfilesMatchLabel string `alloy:"profiles_match_label,attr,optional"`

	// List of labels to copy from discovered targets to profiles. If empty, all labels will be copied.
	LabelsToCopy []string `alloy:"labels_to_copy,attr,optional"`

	// Where to forward profiles after enrichment
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`
}

type Exports struct {
	Receiver pyroscope.Appendable `alloy:"receiver,attr"`
}

type Component struct {
	opts    component.Options
	args    Arguments
	exports Exports

	mut          sync.RWMutex
	targetsCache map[string]labels.Labels
	cacheMutex   sync.RWMutex

	fanout *pyroscope.Fanout
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:         opts,
		args:         args,
		targetsCache: make(map[string]labels.Labels),
		fanout:       pyroscope.NewFanout(args.ForwardTo, opts.ID, opts.Registerer),
	}

	// Initialize the cache with provided targets
	c.refreshCacheFromTargets(args.Targets)

	// Create the receiver appendable that implements both Append and AppendIngest
	c.exports.Receiver = &enrichAppendable{component: c}

	opts.OnStateChange(c.exports)

	return c, nil
}

// enrichAppendable implements pyroscope.Appendable to handle both Append and AppendIngest
type enrichAppendable struct {
	component *Component
}

func (e *enrichAppendable) Appender() pyroscope.Appender {
	return e
}

func (e *enrichAppendable) Append(ctx context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
	enrichedLabels := e.component.enrichLabels(lbls)
	return e.component.fanout.Appender().Append(ctx, enrichedLabels, samples)
}

func (e *enrichAppendable) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	enrichedLabels := e.component.enrichLabels(profile.Labels)

	// Create enriched profile
	enrichedProfile := &pyroscope.IncomingProfile{
		RawBody:     profile.RawBody,
		ContentType: profile.ContentType,
		URL:         profile.URL,
		Labels:      enrichedLabels,
	}

	return e.component.fanout.Appender().AppendIngest(ctx, enrichedProfile)
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *Component) refreshCacheFromTargets(targets []discovery.Target) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	c.targetsCache = make(map[string]labels.Labels, len(targets))

	for _, target := range targets {
		labelPairs := make([]labels.Label, 0)
		target.ForEachLabel(func(k, v string) bool {
			labelPairs = append(labelPairs, labels.Label{Name: k, Value: v})
			return true
		})

		labelSet := labels.New(labelPairs...)
		if matchValue := labelSet.Get(c.args.TargetMatchLabel); matchValue != "" {
			c.targetsCache[matchValue] = labelSet
		}
	}
}

func (c *Component) enrichLabels(lbls labels.Labels) labels.Labels {
	c.mut.RLock()
	matchLabel := c.args.ProfilesMatchLabel
	if matchLabel == "" {
		matchLabel = c.args.TargetMatchLabel
	}
	labelsToCopy := c.args.LabelsToCopy
	c.mut.RUnlock()

	// Get the source value to match against discovered targets
	sourceValue := lbls.Get(matchLabel)
	if sourceValue == "" {
		return lbls
	}

	// Look up matching target
	c.cacheMutex.RLock()
	targetLabels, found := c.targetsCache[sourceValue]
	c.cacheMutex.RUnlock()

	if !found {
		return lbls
	}

	// Copy labels from target to profile labels
	newLabelsBuilder := labels.NewBuilder(lbls)
	if len(labelsToCopy) == 0 {
		// If no specific labels are requested, copy all labels
		targetLabels.Range(func(lbl labels.Label) {
			newLabelsBuilder.Set(lbl.Name, lbl.Value)
		})
	} else {
		// Copy only requested labels
		for _, labelName := range labelsToCopy {
			if value := targetLabels.Get(labelName); value != "" {
				newLabelsBuilder.Set(labelName, value)
			}
		}
	}

	return newLabelsBuilder.Labels()
}

func (c *Component) Name() string {
	return "pyroscope.enrich"
}

func (c *Component) Ready() bool {
	return true
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	c.args = newArgs
	c.mut.Unlock()

	// Update the targets cache with new targets
	c.refreshCacheFromTargets(newArgs.Targets)

	return nil
}

func (c *Component) Exports() component.Exports {
	return &c.exports
}

func (e *enrichAppendable) Upload(j debuginfo.UploadJob) {
	e.component.fanout.Upload(j)
}

func (e *enrichAppendable) Client() debuginfogrpc.DebuginfoServiceClient {
	return e.component.fanout.Client()
}
