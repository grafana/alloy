package aws_firehose

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/util"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.awsfirehose",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server               *fnet.ServerConfig  `alloy:",squash"`
	AccessKey            alloytypes.Secret   `alloy:"access_key,attr,optional"`
	UseIncomingTimestamp bool                `alloy:"use_incoming_timestamp,attr,optional"`
	ForwardTo            []loki.LogsReceiver `alloy:"forward_to,attr"`
	RelabelRules         relabel.Rules       `alloy:"relabel_rules,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server: fnet.DefaultServerConfig(),
	}
}

// Component is the main type for the `loki.source.awsfirehose` component.
type Component struct {
	opts   component.Options
	logger log.Logger

	metrics       *metrics
	serverMetrics *util.UncheckedCollector

	fanout  *loki.Fanout
	handler loki.LogsBatchReceiver

	mut    sync.Mutex
	args   Arguments
	server *source.Server
}

// New creates a new Component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		metrics:       newMetrics(o.Registerer),
		opts:          o,
		handler:       loki.NewLogsBatchReceiver(),
		fanout:        loki.NewFanout(args.ForwardTo),
		serverMetrics: util.NewUncheckedCollector(nil),

		logger: log.With(o.Logger, "component", "aws_firehose_logs"),
	}

	o.Registerer.MustRegister(c.serverMetrics)

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run starts a routine forwards received message to each destination component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		if c.server != nil {
			c.server.ForceShutdown()
		}
	}()

	loki.ConsumeBatch(ctx, c.handler, c.fanout)
	return nil
}

// Update updates the component with a new configuration, restarting the server if needed.
func (c *Component) Update(args component.Arguments) error {
	var err error
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	if newArgs.Server == nil {
		newArgs.Server = &fnet.ServerConfig{}
	}

	if newArgs.Server.GRPC == nil {
		newArgs.Server.GRPC = &fnet.GRPCConfig{
			ListenPort:    0,
			ListenAddress: "127.0.0.1",
		}
	}

	serverNeedsRestart := c.server.NeedsRestart(newArgs.Server) || c.args.AccessKey != newArgs.AccessKey
	if serverNeedsRestart {
		if c.server != nil {
			c.server.Shutdown()
		}

		registry := prometheus.NewRegistry()
		c.serverMetrics.SetCollector(registry)

		c.server, err = source.NewServer(c.logger, registry, c.handler, source.ServerConfig{
			Namespace:      "loki_source_awsfirehose",
			EntriesWritten: c.metrics.entriesWritten,
			NetConfig:      newArgs.Server,
			LogsConfig: &source.LogsConfig{
				RelabelRules:         relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules),
				UseIncomingTimestamp: newArgs.UseIncomingTimestamp,
			},
		})
		if err != nil {
			return err
		}

		if err = c.server.Run(newRoutes(c.metrics, string(newArgs.AccessKey)), nil); err != nil {
			return err
		}

		c.args = newArgs
		return nil
	}

	if c.server != nil {
		c.server.Update(&source.LogsConfig{
			RelabelRules:         relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules),
			UseIncomingTimestamp: newArgs.UseIncomingTimestamp,
		})
	}

	c.args = newArgs
	return nil
}
