package testcomponents

import (
	"context"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.passthrough",
		Stability: featuregate.StabilityPublicPreview,
		Args:      PassthroughConfig{},
		Exports:   PassthroughExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewPassthrough(opts, args.(PassthroughConfig))
		},
	})
}

// PassthroughConfig configures the testcomponents.passthrough component.
type PassthroughConfig struct {
	Input string        `alloy:"input,attr"`
	Lag   time.Duration `alloy:"lag,attr,optional"`
}

// PassthroughExports describes exported fields for the
// testcomponents.passthrough component.
type PassthroughExports struct {
	Output string `alloy:"output,attr,optional"`
}

// Passthrough implements the testcomponents.passthrough component, where it
// always emits its input as an output.
type Passthrough struct {
	opts component.Options
	log  log.Logger
}

// NewPassthrough creates a new passthrough component.
func NewPassthrough(o component.Options, cfg PassthroughConfig) (*Passthrough, error) {
	t := &Passthrough{opts: o, log: o.Logger}
	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component      = (*Passthrough)(nil)
	_ component.DebugComponent = (*Passthrough)(nil)
)

// Run implements Component.
func (t *Passthrough) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (t *Passthrough) Update(args component.Arguments) error {
	c := args.(PassthroughConfig)

	if c.Lag != 0 {
		level.Info(t.log).Log("msg", "sleeping for lag", "lag", c.Lag)
		time.Sleep(c.Lag)
	}

	level.Info(t.log).Log("msg", "passing through value", "value", c.Input)
	t.opts.OnStateChange(PassthroughExports{Output: c.Input})
	return nil
}

// DebugInfo implements DebugComponent.
func (t *Passthrough) DebugInfo() any {
	// Useless, but for demonstration purposes shows how to export debug
	// information. Real components would want to use something interesting here
	// which allow the user to investigate issues of the internal state of a
	// component.
	return passthroughDebugInfo{
		ComponentVersion: "v0.1-beta.0",
	}
}

type passthroughDebugInfo struct {
	ComponentVersion string `alloy:"component_version,attr"`
}
