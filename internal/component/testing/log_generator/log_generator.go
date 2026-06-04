// Package log_generator emits log lines through the Alloy logger at a
// configurable rate. It is intended for stress-testing the logging {}
// block's destinations (notably the Windows Event Log) — not for use in
// production pipelines.
package log_generator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testing.log_generator",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures testing.log_generator.
type Arguments struct {
	// Rate is the number of log lines emitted per second.
	Rate int `alloy:"rate,attr,optional"`
}

// DefaultArguments holds the default values for Arguments.
var DefaultArguments = Arguments{Rate: 100}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() { *a = DefaultArguments }

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.Rate <= 0 {
		return fmt.Errorf("rate must be > 0")
	}
	return nil
}

// filler approximates the variable-length payload of a typical log line.
// Combined with the timestamp/level/component framing that the logger
// adds, each emitted line is roughly an average-length log entry
// (~150 bytes).
const filler = "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut"

var _ component.Component = (*Component)(nil)

// Component is the testing.log_generator runtime.
type Component struct {
	opts component.Options

	mut     sync.Mutex
	rate    int
	restart chan struct{} // closed by Update to wake Run
}

// New creates a new testing.log_generator component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:    o,
		restart: make(chan struct{}),
	}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// takeRate returns the currently-configured rate and a channel that
// will be closed when Update changes the rate.
func (c *Component) takeRate() (int, <-chan struct{}) {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.rate, c.restart
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	rate, restart := c.takeRate()
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()

	var n uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-restart:
			rate, restart = c.takeRate()
			ticker.Reset(time.Second / time.Duration(rate))
		case <-ticker.C:
			n++
			c.opts.SLogger.Info("stress-test entry", "n", n, "payload", filler)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()
	if c.rate == newArgs.Rate {
		return nil
	}
	c.rate = newArgs.Rate
	// Wake Run so it picks up the new rate. Replace the channel for the
	// next Update.
	close(c.restart)
	c.restart = make(chan struct{})
	return nil
}
