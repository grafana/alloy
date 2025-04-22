package testcomponents

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

// testcomponents.stringer takes in an Alloy value, converts it to a string, and forwards it to the defined receivers.
func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.stringer",
		Stability: featuregate.StabilityPublicPreview,
		Args:      StringerConfig{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewStringer(opts, args.(StringerConfig))
		},
	})
}

type StringerConfig struct {
	InputString *string          `alloy:"input_string,attr,optional"`
	InputInt    *int             `alloy:"input_int,attr,optional"`
	InputFloat  *float64         `alloy:"input_float,attr,optional"`
	InputBool   *bool            `alloy:"input_bool,attr,optional"`
	InputMap    *map[string]any  `alloy:"input_map,attr,optional"`
	InputArray  *[]any           `alloy:"input_array,attr,optional"`
	ForwardTo   []StringReceiver `alloy:"forward_to,attr"`
}

type Stringer struct {
	opts      component.Options
	log       log.Logger
	cfgUpdate chan StringerConfig
}

func NewStringer(o component.Options, cfg StringerConfig) (*Stringer, error) {
	t := &Stringer{
		opts:      o,
		log:       o.Logger,
		cfgUpdate: make(chan StringerConfig, 10),
	}
	return t, nil
}

var (
	_ component.Component = (*Stringer)(nil)
)

func forward(val any, to []StringReceiver) {
	for _, r := range to {
		str := fmt.Sprintf("%#v", val)
		r.Receive(str)
	}
}

func (s *Stringer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case cfg := <-s.cfgUpdate:
			// Send the new values to the receivers
			if cfg.InputString != nil {
				forward(*cfg.InputString, cfg.ForwardTo)
			}
			if cfg.InputInt != nil {
				forward(*cfg.InputInt, cfg.ForwardTo)
			}
			if cfg.InputFloat != nil {
				forward(*cfg.InputFloat, cfg.ForwardTo)
			}
			if cfg.InputBool != nil {
				forward(*cfg.InputBool, cfg.ForwardTo)
			}
			if cfg.InputArray != nil {
				forward(*cfg.InputArray, cfg.ForwardTo)
			}
			if cfg.InputMap != nil {
				forward(*cfg.InputMap, cfg.ForwardTo)
			}
		}
	}
}

// Update implements Component.
func (s *Stringer) Update(args component.Arguments) error {
	cfg := args.(StringerConfig)
	s.cfgUpdate <- cfg
	return nil
}
