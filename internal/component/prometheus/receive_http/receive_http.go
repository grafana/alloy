package receive_http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloyprom "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.receive_http",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server    *fnet.ServerConfig   `alloy:",squash"`
	ForwardTo []storage.Appendable `alloy:"forward_to,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Server: fnet.DefaultServerConfig(),
	}
}

type Component struct {
	opts               component.Options
	handler            http.Handler
	fanout             *alloyprom.Fanout
	uncheckedCollector *util.UncheckedCollector

	updateMut sync.RWMutex
	args      Arguments
	server    *fnet.TargetServer
}

func New(opts component.Options, args Arguments) (*Component, error) {
	service, err := opts.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)
	fanout := alloyprom.NewFanout(args.ForwardTo, opts.Registerer, ls)

	uncheckedCollector := util.NewUncheckedCollector(nil)
	opts.Registerer.MustRegister(uncheckedCollector)

	// TODO: Make these configurable in the future?
	supportedRemoteWriteProtoMsgs := config.RemoteWriteProtoMsgs{config.RemoteWriteProtoMsgV1}
	ingestCTZeroSample := false

	c := &Component{
		opts: opts,
		handler: remote.NewWriteHandler(
			slog.New(logging.NewSlogGoKitHandler(opts.Logger)),
			opts.Registerer,
			fanout,
			supportedRemoteWriteProtoMsgs,
			ingestCTZeroSample,
		),
		fanout:             fanout,
		uncheckedCollector: uncheckedCollector,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run satisfies the Component interface.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.updateMut.Lock()
		defer c.updateMut.Unlock()
		c.shutdownServer()
	}()

	<-ctx.Done()
	level.Info(c.opts.Logger).Log("msg", "terminating due to context done")
	return nil
}

// Update satisfies the Component interface.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.updateMut.Lock()
	defer c.updateMut.Unlock()

	serverNeedsUpdate := !reflect.DeepEqual(c.args.Server, newArgs.Server)
	if !serverNeedsUpdate {
		c.args = newArgs
		return nil
	}
	c.shutdownServer()

	s, err := c.createNewServer(newArgs)
	if err != nil {
		return err
	}
	c.server = s

	err = c.server.MountAndRun(func(router *mux.Router) {
		router.Path("/api/v1/metrics/write").Methods("POST").Handler(c.handler)
	})
	if err != nil {
		return err
	}

	c.args = newArgs
	return nil
}

func (c *Component) createNewServer(args Arguments) (*fnet.TargetServer, error) {
	// [server.Server] registers new metrics every time it is created. To
	// avoid issues with re-registering metrics with the same name, we create a
	// new registry for the server every time we create one, and pass it to an
	// unchecked collector to bypass uniqueness checking.
	serverRegistry := prometheus.NewRegistry()
	c.uncheckedCollector.SetCollector(serverRegistry)

	s, err := fnet.NewTargetServer(
		c.opts.Logger,
		"prometheus_receive_http",
		serverRegistry,
		args.Server,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %v", err)
	}

	return s, nil
}

// shutdownServer will shut down the currently used server.
// It is not goroutine-safe and an updateMut write lock must be held when it's called.
func (c *Component) shutdownServer() {
	if c.server != nil {
		c.server.StopAndShutdown()
		c.server = nil
	}
}
