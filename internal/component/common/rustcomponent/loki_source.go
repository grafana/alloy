package rustcomponent

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"math"
	"sync"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

// LokiSourceArguments are the arguments every Rust loki source has. Add them
// to a component's arguments as a field tagged `alloy:",squash"`.
type LokiSourceArguments struct {
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
	Labels    map[string]string   `alloy:"labels,attr,optional"`
}

// LokiSourceArgs are the arguments of a Rust loki source. Fields tagged
// `rust:"<type>"` are passed to Rust, see EncodeArgs.
type LokiSourceArgs interface {
	LokiSource() LokiSourceArguments
}

// RegisterLokiSource registers the Rust loki source called name. The Rust
// side must be registered under the same name.
func RegisterLokiSource[A LokiSourceArgs](name string, stability featuregate.Stability) {
	var args A
	component.Register(component.Registration{
		Name:      name,
		Stability: stability,
		Args:      args,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return newLokiSource(name, opts, args.(A))
		},
	})
}

type lokiSource[A LokiSourceArgs] struct {
	name    string
	opts    component.Options
	fanout  *loki.FanoutV2
	restart chan struct{}

	mut    sync.Mutex
	raw    []byte
	labels map[string]string
}

func newLokiSource[A LokiSourceArgs](name string, opts component.Options, args A) (*lokiSource[A], error) {
	svc, err := opts.GetServiceData(lokipipelinev2.ServiceName)
	if err != nil {
		return nil, err
	}
	if !svc.(lokipipelinev2.Data).PipelineV2Enabled() {
		return nil, errors.New(name + " requires Loki pipeline V2")
	}

	raw, err := EncodeArgs(args)
	if err != nil {
		return nil, err
	}

	return &lokiSource[A]{
		name:    name,
		opts:    opts,
		fanout:  loki.NewFanoutV2(args.LokiSource().ForwardTo),
		restart: make(chan struct{}, 1),
		raw:     raw,
		labels:  args.LokiSource().Labels,
	}, nil
}

func (c *lokiSource[A]) Run(ctx context.Context) error {
	for {
		c.mut.Lock()
		raw, labels := c.raw, c.labels
		c.mut.Unlock()

		src, err := NewSource(c.name, raw)
		if err != nil {
			return err
		}
		srcCtx, cancel := context.WithCancel(ctx)

		var wg sync.WaitGroup
		wg.Go(src.Run)
		wg.Go(func() { c.drain(srcCtx, src, labels) })

		select {
		case <-ctx.Done():
		case <-c.restart:
		}

		cancel()
		src.Close()
		wg.Wait()
		src.Free()

		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *lokiSource[A]) drain(ctx context.Context, src *Source, labels map[string]string) {
	ls := make(model.LabelSet, len(labels))
	for k, v := range labels {
		ls[model.LabelName(k)] = model.LabelValue(v)
	}

	// Flush once per Rust batch rather than by size.
	batcher := loki.NewStreamBatcherV2(c.opts.Logger, c.fanout, ls, math.MaxInt)

	// Keep receiving after ctx is canceled, the Rust side can only stop once
	// everything it sent has been received.
	for src.Recv(func(e push.Entry) { batcher.Add(e) }) {
		if err := batcher.Flush(ctx); err != nil && ctx.Err() == nil {
			c.opts.Logger.Error("failed to send entries", "err", err)
		}
	}
}

func (c *lokiSource[A]) Update(args component.Arguments) error {
	newArgs := args.(A)
	c.fanout.UpdateChildren(newArgs.LokiSource().ForwardTo)

	raw, err := EncodeArgs(newArgs)
	if err != nil {
		return err
	}
	labels := newArgs.LokiSource().Labels

	c.mut.Lock()
	changed := !bytes.Equal(raw, c.raw) || !maps.Equal(labels, c.labels)
	c.raw, c.labels = raw, labels
	c.mut.Unlock()

	if changed {
		select {
		case c.restart <- struct{}{}:
		default:
		}
	}
	return nil
}
