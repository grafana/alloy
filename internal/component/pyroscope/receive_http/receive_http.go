package receive_http

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"reflect"
	"sync"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	// defaultMaxConnLimit defines the maximum number of simultaneous HTTP connections
	defaultMaxConnLimit = 100
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.receive_http",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server    *fnet.ServerConfig     `alloy:",squash"`
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	serverConfig := fnet.DefaultServerConfig()
	if serverConfig.HTTP.ConnLimit == 0 {
		serverConfig.HTTP.ConnLimit = defaultMaxConnLimit
	}
	*a = Arguments{
		Server: serverConfig,
	}
}

type Component struct {
	opts        component.Options
	server      *fnet.TargetServer
	appendables []pyroscope.Appendable
	mut         sync.Mutex
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:        opts,
		appendables: args.ForwardTo,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.shutdownServer()
	}()

	<-ctx.Done()
	level.Info(c.opts.Logger).Log("msg", "terminating due to context done")
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.mut.Lock()
	defer c.mut.Unlock()

	c.appendables = newArgs.ForwardTo

	// if no server config provided, we'll use defaults
	if newArgs.Server == nil {
		newArgs.Server = fnet.DefaultServerConfig()
	}

	// Only apply default max connections limit if using default config
	if newArgs.Server.HTTP.ConnLimit == 0 {
		newArgs.Server.HTTP.ConnLimit = defaultMaxConnLimit
	}

	if newArgs.Server.HTTP == nil {
		newArgs.Server.HTTP = &fnet.HTTPConfig{
			ListenPort:    0,
			ListenAddress: "127.0.0.1",
		}
	}

	serverNeedsRestarting := c.server == nil || !reflect.DeepEqual(c.server, *newArgs.Server.HTTP)
	if !serverNeedsRestarting {
		return nil
	}

	c.shutdownServer()

	srv, err := fnet.NewTargetServer(c.opts.Logger, "pyroscope_receive_http", c.opts.Registerer, newArgs.Server)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	c.server = srv

	return c.server.MountAndRun(func(router *mux.Router) {
		router.HandleFunc("/ingest", c.handleIngest).Methods(http.MethodPost)
	})
}

func (c *Component) handleIngest(w http.ResponseWriter, r *http.Request) {
	c.mut.Lock()
	appendables := c.appendables
	c.mut.Unlock()

	// Create a pipe for each appendable
	pipeWriters := make([]io.Writer, len(appendables))
	pipeReaders := make([]io.Reader, len(appendables))
	for i := range appendables {
		pr, pw := io.Pipe()
		pipeReaders[i] = pr
		pipeWriters[i] = pw
	}
	mw := io.MultiWriter(pipeWriters...)

	// Create an errgroup with the timeout context
	g, ctx := errgroup.WithContext(r.Context())

	// Start copying the request body to all pipes
	g.Go(func() error {
		defer func() {
			for _, pw := range pipeWriters {
				pw.(io.WriteCloser).Close()
			}
		}()
		_, err := io.Copy(mw, r.Body)
		return err
	})

	// Process each appendable
	for i, appendable := range appendables {
		g.Go(func() error {
			defer pipeReaders[i].(io.ReadCloser).Close()

			profile := &pyroscope.IncomingProfile{
				Body:    io.NopCloser(pipeReaders[i]),
				Headers: r.Header.Clone(),
				URL:     r.URL,
			}

			err := appendable.Appender().AppendIngest(ctx, profile)
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "Failed to append profile", "appendable", i, "err", err)
				return err
			}
			level.Debug(c.opts.Logger).Log("msg", "Profile appended successfully", "appendable", i)
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		var writeErr *write.PyroscopeWriteError
		if errors.As(err, &writeErr) {
			http.Error(w, http.StatusText(writeErr.StatusCode), writeErr.StatusCode)
		} else {
			level.Error(c.opts.Logger).Log("msg", "Failed to process request", "err", err)
			http.Error(w, "Failed to process request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Component) shutdownServer() {
	if c.server != nil {
		c.server.StopAndShutdown()
		c.server = nil
	}
}
