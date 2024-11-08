package dbo11y

import (
	"context"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "grafanacloud.dbo11y",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	SomeArg   string              `alloy:"somearg,attr,optional"`
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`
}

type Component struct {
	log       log.Logger
	mut       sync.RWMutex
	receivers []loki.LogsReceiver
	handler   loki.LogsReceiver
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		log:       opts.Logger,
		receivers: args.ForwardTo,
		handler:   loki.NewLogsReceiver(),
	}

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.receivers {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	go func() {
		entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})
		entryHandler.Chan() <- loki.Entry{
			Labels: model.LabelSet{"lbl": "val"},
			Entry: logproto.Entry{
				Timestamp: time.Unix(0, time.Now().UnixNano()),
				Line:      args.(Arguments).SomeArg,
			},
		}
	}()

	return nil
}
