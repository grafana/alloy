package relabel

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.relabel",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the discovery.relabel component.
type Arguments struct {
	// Targets contains the input 'targets' passed by a service discovery component.
	Targets []discovery.Target `alloy:"targets,attr"`

	// The relabelling rules to apply to each target's label set.
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`
}

// Exports holds values which are exported by the discovery.relabel component.
type Exports struct {
	Output []discovery.Target  `alloy:"output,attr"`
	Rules  alloy_relabel.Rules `alloy:"rules,attr"`
}

// Component implements the discovery.relabel component.
type Component struct {
	opts component.Options

	mut sync.RWMutex
	rcs []*relabel.Config

	debugDataPublisher livedebugging.DebugDataPublisher
}

var _ component.Component = (*Component)(nil)
var _ component.LiveDebugging = (*Component)(nil)

// New creates a new discovery.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts:               o,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Call to Update() to set the output once at the start
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	targets := make([]discovery.Target, 0, len(newArgs.Targets))
	relabelConfigs := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)
	c.rcs = relabelConfigs

	lbls := make(labels.Labels, 0, 32)
	lblBuilder := labels.NewBuilder(lbls)

	for _, t := range newArgs.Targets {
		// set existing label in the builder
		lbls = discoveryTargetToLabels(t, lbls)
		lblBuilder.Reset(lbls)

		keep := relabel.ProcessBuilder(lblBuilder, relabelConfigs...)
		if keep {
			targets = append(targets, builderToDiscoveryTarget(lblBuilder, len(t)))
		}
		componentID := livedebugging.ComponentID(c.opts.ID)
		if c.debugDataPublisher.IsActive(componentID) {
			lblAfter := lblBuilder.Labels().String()
			c.debugDataPublisher.Publish(componentID, fmt.Sprintf("%s => %s", lbls.String(), lblAfter))
		}
	}

	c.opts.OnStateChange(Exports{
		Output: targets,
		Rules:  newArgs.RelabelConfigs,
	})

	return nil
}

func (c *Component) LiveDebugging(_ int) {}

func discoveryTargetToLabels(t discovery.Target, lbls labels.Labels) labels.Labels {
	if cap(lbls) < len(t) {
		lbls = make([]labels.Label, 0, len(t))
	} else {
		lbls = lbls[:len(t)]
	}
	for k, v := range t {
		lbls = append(lbls, labels.Label{Name: k, Value: v})
	}
	return lbls
}

func builderToDiscoveryTarget(b *labels.Builder, size int) discovery.Target {
	res := make(map[string]string, size)
	b.Range(func(l labels.Label) {
		res[l.Name] = l.Value
	})
	return res
}
