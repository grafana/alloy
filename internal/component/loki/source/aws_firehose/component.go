package aws_firehose

import (
	"context"
	"reflect"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/aws_firehose/internal"
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
	RelabelRules         alloy_relabel.Rules `alloy:"relabel_rules,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server: fnet.DefaultServerConfig(),
	}
}

// Component is the main type for the `loki.source.awsfirehose` component.
type Component struct {
	// mut controls concurrent access to fanout
	mut       sync.RWMutex
	receivers []loki.LogsReceiver

	// destination is the main destination where the TargetServer writes received log entries to
	destination loki.LogsReceiver
	rbs         []*relabel.Config

	server *fnet.TargetServer

	opts component.Options
	args Arguments

	// utils
	serverMetrics  *util.UncheckedCollector
	handlerMetrics *internal.Metrics
	logger         log.Logger
}

// New creates a new Component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:           o,
		destination:    loki.NewLogsReceiver(),
		receivers:      args.ForwardTo,
		serverMetrics:  util.NewUncheckedCollector(nil),
		handlerMetrics: internal.NewMetrics(o.Registerer),

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
		c.shutdownServer()
	}()

	for {
		// NOTE: if we failed to receive entry that means that context was
		// canceled and we should exit component.
		entry, ok := c.destination.Recv(ctx)
		if !ok {
			return nil
		}

		c.mut.RLock()
		for _, receiver := range c.receivers {
			// NOTE: if we did not send the entry that mean that context was
			// canceled and we should exit component.
			if ok := receiver.Send(ctx, entry); !ok {
				c.mut.RUnlock()
				return nil
			}
		}
		c.mut.RUnlock()
	}
}

// Update updates the component with a new configuration, restarting the server if needed.
func (c *Component) Update(args component.Arguments) error {
	var err error
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.receivers = newArgs.ForwardTo

	var newRelabels []*relabel.Config = nil
	// first condition to consider if the handler needs to be updated is if the UseIncomingTimestamp field
	// changed
	var handlerNeedsUpdate = c.args.UseIncomingTimestamp != newArgs.UseIncomingTimestamp

	// then, if the relabel rules changed
	if len(newArgs.RelabelRules) > 0 {
		handlerNeedsUpdate = true
		newRelabels = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	} else if len(c.rbs) > 0 && len(newArgs.RelabelRules) == 0 {
		// nil out relabel rules if they need to be cleared
		handlerNeedsUpdate = true
	}

	if c.args.AccessKey != newArgs.AccessKey {
		handlerNeedsUpdate = true
	}

	// Since the handler is created ad-hoc for the server, and the handler depends on the relabels
	// consider this as a cause for server restart as well. Much simpler than adding a lock on the
	// handler and doing the relabel rules change on the fly
	serverNeedsUpdate := !reflect.DeepEqual(c.args.Server, newArgs.Server)
	if !serverNeedsUpdate && !handlerNeedsUpdate {
		c.args = newArgs
		return nil
	}

	c.shutdownServer()

	// update relabel rules in component if needed
	if handlerNeedsUpdate {
		c.rbs = newRelabels
	}

	r := strings.NewReplacer(".", "_", "/", "_")
	jobName := r.Replace(c.opts.ID)

	registry := prometheus.NewRegistry()
	c.serverMetrics.SetCollector(registry)

	c.server, err = fnet.NewTargetServer(c.logger, jobName, registry, newArgs.Server)
	if err != nil {
		return err
	}

	if err = c.server.MountAndRun(func(router *mux.Router) {
		// re-create handler when server is re-computed
		handler := internal.NewHandler(c, c.logger, c.handlerMetrics, c.rbs, newArgs.UseIncomingTimestamp, string(newArgs.AccessKey))
		router.Path("/awsfirehose/api/v1/push").Methods("POST").Handler(handler)
	}); err != nil {
		return err
	}

	c.args = newArgs
	return nil
}

// Send implements internal.Sender so that the component is able to receive logs decoded by the handler.
func (c *Component) Send(ctx context.Context, entry loki.Entry) {
	c.destination.Chan() <- entry
}

// shutdownServer will shut down the currently used server.
// It is not goroutine-safe and mut write lock must be held when it's called.
func (c *Component) shutdownServer() {
	if c.server != nil {
		c.server.StopAndShutdown()
		c.server = nil
	}
}
