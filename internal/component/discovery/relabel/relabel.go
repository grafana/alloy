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

	for _, t := range newArgs.Targets {
		var (
			relabelled discovery.Target
			builder    = discovery.NewTargetBuilderFrom(t)
			keep       = alloy_relabel.ProcessBuilder(builder, newArgs.RelabelConfigs...)
		)
		if keep {
			relabelled = builder.Target()
			targets = append(targets, relabelled)
		}
		componentID := livedebugging.ComponentID(c.opts.ID)
		c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
			componentID,
			livedebugging.Target,
			1,
			func() string { return fmt.Sprintf("%s => %s", t, relabelled) },
		))
	}

	c.opts.OnStateChange(Exports{
		Output: targets,
		Rules:  newArgs.RelabelConfigs,
	})

	return nil
}

func (c *Component) LiveDebugging() {}
