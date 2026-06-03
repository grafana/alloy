package receiver

import (
	"context"
	"log/slog"
	"reflect"
	"sync"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/slogadapter"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/sigil-sdk/go/proto/sigil/wire"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	component.Register(component.Registration{
		Name:      "sigil.receive",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	logger             *slog.Logger
	handler            *handler
	uncheckedCollector *util.UncheckedCollector

	mut          sync.Mutex
	server       *fnet.TargetServer
	serverConfig *fnet.ServerConfig
}

func New(opts component.Options, args Arguments) (*Component, error) {
	uncheckedCollector := util.NewUncheckedCollector(nil)
	opts.Registerer.MustRegister(uncheckedCollector)

	h := newHandler(opts.SLogger, args.ForwardTo, int64(args.MaxRequestBodySize))

	c := &Component{
		logger:             opts.SLogger,
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
	c.logger.Info("terminating sigil.receive")
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

	serverNeedsRestarting := !reflect.DeepEqual(c.serverConfig, args.Server)
	if !serverNeedsRestarting {
		return nil
	}

	c.shutdownServer()
	c.serverConfig = nil

	serverRegistry := prometheus.NewRegistry()
	c.uncheckedCollector.SetCollector(serverRegistry)

	srv, err := fnet.NewTargetServer(slogadapter.GoKit(c.logger.Handler()), "sigil_receive_http", serverRegistry, args.Server)
	if err != nil {
		return err
	}

	if err := srv.MountAndRun(func(router *mux.Router) {
		router.Handle(wire.GenerationExportHTTPPath, c.handler).Methods("POST")
	}); err != nil {
		return err
	}

	c.server = srv
	c.serverConfig = args.Server
	return nil
}

func (c *Component) shutdownServer() {
	if c.server != nil {
		c.server.StopAndShutdown()
		c.server = nil
	}
}
