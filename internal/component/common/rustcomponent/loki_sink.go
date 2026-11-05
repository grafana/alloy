package rustcomponent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

// LokiSinkExports are the exports of every Rust loki sink.
type LokiSinkExports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// RegisterLokiSink registers the Rust loki sink called name. The Rust side
// must be registered under the same name. Fields of A tagged
// `rust:"<type>"` are passed to Rust, see EncodeArgs.
func RegisterLokiSink[A component.Arguments](name string, stability featuregate.Stability) {
	var args A
	component.Register(component.Registration{
		Name:      name,
		Stability: stability,
		Args:      args,
		Exports:   LokiSinkExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return newLokiSink(name, opts, args.(A))
		},
	})
}

var _ loki.Consumer = (*lokiSink[struct{}])(nil)

type lokiSink[A component.Arguments] struct {
	name string
	opts component.Options

	mut     sync.RWMutex
	sink    *Sink
	raw     []byte
	stopped bool
}

func newLokiSink[A component.Arguments](name string, opts component.Options, args A) (*lokiSink[A], error) {
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
	sink, err := NewSink(name, raw)
	if err != nil {
		return nil, err
	}

	c := &lokiSink[A]{
		name: name,
		opts: opts,
		sink: sink,
		raw:  raw,
	}
	opts.OnStateChange(LokiSinkExports{Receiver: loki.NewLogsReceiverV2(c, opts.ID)})
	return c, nil
}

func (c *lokiSink[A]) Run(ctx context.Context) error {
	<-ctx.Done()

	c.mut.Lock()
	defer c.mut.Unlock()
	c.stopped = true
	c.sink.Close()
	c.sink.Free()
	return nil
}

func (c *lokiSink[A]) Update(args component.Arguments) error {
	raw, err := EncodeArgs(args.(A))
	if err != nil {
		return err
	}

	c.mut.RLock()
	changed := !bytes.Equal(raw, c.raw)
	c.mut.RUnlock()
	if !changed {
		return nil
	}

	sink, err := NewSink(c.name, raw)
	if err != nil {
		return err
	}

	c.mut.Lock()
	defer c.mut.Unlock()
	if c.stopped {
		sink.Free()
		return nil
	}
	old := c.sink
	c.sink, c.raw = sink, raw
	old.Close()
	old.Free()
	return nil
}

func (c *lokiSink[A]) Consume(_ context.Context, batch loki.Batch) error {
	c.mut.RLock()
	defer c.mut.RUnlock()
	if c.stopped {
		return fmt.Errorf("%s: %w", c.opts.ID, loki.ErrConsumerStopped)
	}

	if err := c.sink.Consume(batch); err != nil {
		return fmt.Errorf("%s: %w", c.opts.ID, err)
	}
	return nil
}
