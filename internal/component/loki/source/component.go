package source

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type Arguments interface {
	Receivers() []loki.LogsReceiver
}

// Component is a generic structure that should be used as a base for all source components.
// It is responsible for managing the lifecycle of the component and the sources it manages.
type Component struct {
	opts component.Options

	factory SourceFactory

	// recv is the channel source component will consume from
	// and is static for the lifetime of the component.
	recv loki.LogsReceiver

	targetsMut     sync.RWMutex
	targets        []Source
	targetsUpdated chan struct{}

	forwardTo    []loki.LogsReceiver
	forwardToMut sync.RWMutex

	pos positions.Positions

	stopping atomic.Bool
}

var (
	_ Host                = (*Component)(nil)
	_ component.Component = (*Component)(nil)
)

func New(opts component.Options, args Arguments, factory SourceFactory) (component.Component, error) {
	err := os.MkdirAll(opts.DataPath, 0750)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	pos, err := positions.New(opts.Logger, positions.Config{
		SyncPeriod:        10 * time.Second,
		PositionsFile:     filepath.Join(opts.DataPath, "positions.yml"),
		IgnoreInvalidYaml: false,
		ReadOnly:          false,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create position file: %w", err)
	}

	c := &Component{
		opts:           opts,
		factory:        factory,
		recv:           loki.NewLogsReceiver(),
		pos:            pos,
		targetsUpdated: make(chan struct{}, 1),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	r := runner.New(func(t Source) runner.Worker {
		return t
	})

	defer func() {
		c.stopping.Store(true)
		// We stop position file first because we are draning into nothing
		// so we don't want to update position for these entries.
		c.pos.Stop()
		ctx, cancel := context.WithCancel(context.Background())
		go c.drain(ctx)
		r.Stop()
		cancel()
	}()

	var wg sync.WaitGroup
	// Start consumer loop to consume entries from the receiver and forward.
	// This loop will run until the context is done.
	wg.Go(func() { c.consume(ctx) })

	// Start scheduler loop to schedule sources.
	// This loop will run until the context is done.
	wg.Go(func() { c.schedule(ctx, r) })
	wg.Wait()

	return nil
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.forwardToMut.RLock()
	if receiversChanged(c.forwardTo, newArgs.Receivers()) {
		// Upgrade lock to write.
		c.forwardToMut.RUnlock()
		c.forwardToMut.Lock()
		c.forwardTo = newArgs.Receivers()
		c.forwardToMut.Unlock()
	} else {
		c.forwardToMut.RUnlock()
	}

	c.targetsMut.Lock()
	c.targets = c.factory.Sources(c, args)
	c.targetsMut.Unlock()

	select {
	case c.targetsUpdated <- struct{}{}:
	default:
	}

	return nil
}

func (c *Component) Reciever() loki.LogsReceiver {
	return c.recv
}

func (c *Component) Positions() positions.Positions {
	return c.pos
}

func (c *Component) Stopping() bool {
	return c.stopping.Load()
}

// drain is used to drain the receiver channel when the component is stopping.
func (c *Component) drain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.recv.Chan():
		}
	}
}

// consume is used to consume entries from the receiver and forward them to the other receivers.
func (c *Component) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-c.recv.Chan():
			c.forward(ctx, e)
		}
	}
}

func (c *Component) forward(ctx context.Context, e loki.Entry) {
	c.forwardToMut.RLock()
	defer c.forwardToMut.RUnlock()
	for _, f := range c.forwardTo {
		select {
		case <-ctx.Done():
			return
		case f.Chan() <- e:
		}
	}
}

// schedule is used to schedule sources.
func (c *Component) schedule(ctx context.Context, r *runner.Runner[Source]) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.targetsUpdated:
			c.targetsMut.RLock()
			level.Debug(c.opts.Logger).Log("msg", "updating targets", "targets", len(c.targets))
			if err := r.ApplyTasks(ctx, c.targets); err != nil {
				if !errors.Is(err, context.Canceled) {
					level.Error(c.opts.Logger).Log("msg", "failed to apply tasks", "err", err)
				}
			} else {
				level.Debug(c.opts.Logger).Log("msg", "workers successfully updated", "workers", len(r.Workers()))
			}

			c.targetsMut.RUnlock()
		}
	}
}

func receiversChanged(prev, next []loki.LogsReceiver) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}
