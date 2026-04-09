package receiver

import (
	"context"
	"reflect"
	"sync"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	component.Register(component.Registration{
		Name:      "sigil.receiver",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	logger             log.Logger
	handler            *handler
	uncheckedCollector *util.UncheckedCollector

	mut          sync.Mutex
	server       *fnet.TargetServer
	serverConfig *fnet.HTTPConfig
}

func New(opts component.Options, args Arguments) (*Component, error) {
	uncheckedCollector := util.NewUncheckedCollector(nil)
	opts.Registerer.MustRegister(uncheckedCollector)

	m := newMetrics(opts.Registerer)
	h := newHandler(opts.Logger, m, args.ForwardTo, int64(args.MaxRequestBodySize))

	c := &Component{
		logger:             opts.Logger,
		handler:            h,
		uncheckedCollector: uncheckedCollector,
	}

	if err := c.update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.shutdownServer()
	}()

	<-ctx.Done()
	level.Info(c.logger).Log("msg", "terminating sigil.receiver")
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	return c.update(args.(Arguments))
}

func (c *Component) update(args Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.handler.update(args.ForwardTo, int64(args.MaxRequestBodySize))

	serverNeedsRestarting := !reflect.DeepEqual(c.serverConfig, args.Server.HTTP)
	if !serverNeedsRestarting {
		return nil
	}

	c.shutdownServer()
	c.serverConfig = nil

	serverRegistry := prometheus.NewRegistry()
	c.uncheckedCollector.SetCollector(serverRegistry)

	srv, err := fnet.NewTargetServer(c.logger, "sigil_receiver", serverRegistry, args.Server)
	if err != nil {
		return err
	}
	c.server = srv
	c.serverConfig = args.Server.HTTP

	return c.server.MountAndRun(func(router *mux.Router) {
		router.Handle("/api/v1/generations:export", c.handler).Methods("POST")
	})
}

func (c *Component) shutdownServer() {
	if c.server != nil {
		c.server.StopAndShutdown()
		c.server = nil
	}
}
