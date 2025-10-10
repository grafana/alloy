package api

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/alecthomas/units"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/api/internal/lokipush"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/util"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.api",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server               *fnet.ServerConfig  `alloy:",squash"`
	ForwardTo            []loki.LogsReceiver `alloy:"forward_to,attr"`
	Labels               map[string]string   `alloy:"labels,attr,optional"`
	RelabelRules         relabel.Rules       `alloy:"relabel_rules,attr,optional"`
	UseIncomingTimestamp bool                `alloy:"use_incoming_timestamp,attr,optional"`
	MaxSendMessageSize   units.Base2Bytes    `alloy:"max_send_message_size,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server:             fnet.DefaultServerConfig(),
		MaxSendMessageSize: 100 * units.MiB,
	}
}

func (a *Arguments) labelSet() model.LabelSet {
	labelSet := make(model.LabelSet, len(a.Labels))
	for k, v := range a.Labels {
		labelSet[model.LabelName(k)] = model.LabelValue(v)
	}
	return labelSet
}

type Component struct {
	opts               component.Options
	entriesChan        chan loki.Entry
	uncheckedCollector *util.UncheckedCollector

	serverMut sync.Mutex
	server    *lokipush.PushAPIServer

	// Use separate receivers mutex to address potential deadlock when Update drains the current server.
	// e.g. https://github.com/grafana/agent/issues/3391
	receiversMut sync.RWMutex
	receivers    []loki.LogsReceiver
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:               opts,
		entriesChan:        make(chan loki.Entry),
		receivers:          args.ForwardTo,
		uncheckedCollector: util.NewUncheckedCollector(nil),
	}
	opts.Registerer.MustRegister(c.uncheckedCollector)
	err := c.Update(args)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Component) Run(ctx context.Context) (err error) {
	defer c.stop()

	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-c.entriesChan:
			c.receiversMut.RLock()
			for _, receiver := range c.receivers {
				// NOTE: if we did not send the entry that mean that context was
				// canceled and we should exit component.
				if ok := receiver.Send(ctx, entry); !ok {
					c.receiversMut.RUnlock()
					return nil
				}
			}
			c.receiversMut.RUnlock()
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	newArgs, ok := args.(Arguments)
	if !ok {
		return fmt.Errorf("invalid type of arguments: %T", args)
	}

	// if no server config provided, we'll use defaults
	if newArgs.Server == nil {
		newArgs.Server = &fnet.ServerConfig{}
	}
	// to avoid port conflicts, if no GRPC is configured, make sure we use a random port
	// also, use localhost IP, so we don't require root to run.
	if newArgs.Server.GRPC == nil {
		newArgs.Server.GRPC = &fnet.GRPCConfig{
			ListenPort:    0,
			ListenAddress: "127.0.0.1",
		}
	}

	c.receiversMut.Lock()
	c.receivers = newArgs.ForwardTo
	c.receiversMut.Unlock()

	c.serverMut.Lock()
	defer c.serverMut.Unlock()
	serverNeedsRestarting := c.server == nil || !reflect.DeepEqual(c.server.ServerConfig(), *newArgs.Server)
	if serverNeedsRestarting {
		if c.server != nil {
			c.server.Shutdown()
		}

		// [server.Server] registers new metrics every time it is created. To
		// avoid issues with re-registering metrics with the same name, we create a
		// new registry for the server every time we create one, and pass it to an
		// unchecked collector to bypass uniqueness checking.
		serverRegistry := prometheus.NewRegistry()
		c.uncheckedCollector.SetCollector(serverRegistry)

		var err error
		c.server, err = lokipush.NewPushAPIServer(c.opts.Logger, newArgs.Server, loki.NewEntryHandler(c.entriesChan, func() {}), serverRegistry, int64(newArgs.MaxSendMessageSize))
		if err != nil {
			return fmt.Errorf("failed to create embedded server: %v", err)
		}
		err = c.server.Run()
		if err != nil {
			return fmt.Errorf("failed to run embedded server: %v", err)
		}
	}

	c.server.SetLabels(newArgs.labelSet())
	c.server.SetRelabelRules(newArgs.RelabelRules)
	c.server.SetKeepTimestamp(newArgs.UseIncomingTimestamp)

	return nil
}

func (c *Component) stop() {
	c.serverMut.Lock()
	defer c.serverMut.Unlock()
	if c.server != nil {
		c.server.Shutdown()
		c.server = nil
	}
}
