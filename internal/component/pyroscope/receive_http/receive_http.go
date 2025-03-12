package receive_http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/grafana/pyroscope/api/model/labelset"
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
	opts               component.Options
	server             *fnet.TargetServer
	uncheckedCollector *util.UncheckedCollector
	appendables        []pyroscope.Appendable
	mut                sync.Mutex
}

func New(opts component.Options, args Arguments) (*Component, error) {
	uncheckedCollector := util.NewUncheckedCollector(nil)
	opts.Registerer.MustRegister(uncheckedCollector)

	c := &Component{
		opts:               opts,
		uncheckedCollector: uncheckedCollector,
		appendables:        args.ForwardTo,
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

	// [server.Server] registers new metrics every time it is created. To
	// avoid issues with re-registering metrics with the same name, we create a
	// new registry for the server every time we create one, and pass it to an
	// unchecked collector to bypass uniqueness checking.
	serverRegistry := prometheus.NewRegistry()
	c.uncheckedCollector.SetCollector(serverRegistry)

	srv, err := fnet.NewTargetServer(c.opts.Logger, "pyroscope_receive_http", serverRegistry, newArgs.Server)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	c.server = srv

	return c.server.MountAndRun(func(router *mux.Router) {
		// this mounts the og pyroscope ingest API, mostly used by SDKs
		router.HandleFunc("/ingest", c.handleIngest).Methods(http.MethodPost)

		// mount connect go pushv1
		pathPush, handlePush := pushv1connect.NewPusherServiceHandler(c)
		router.PathPrefix(pathPush).Handler(handlePush).Methods(http.MethodPost)
	})
}

func setLabelBuilderFromAPI(lb *labels.Builder, api []*typesv1.LabelPair) {
	for i := range api {
		lb.Set(api[i].Name, api[i].Value)
	}
}

func apiToAlloySamples(api []*pushv1.RawSample) []*pyroscope.RawSample {
	var (
		alloy = make([]*pyroscope.RawSample, len(api))
	)
	for i := range alloy {
		alloy[i] = &pyroscope.RawSample{
			RawProfile: api[i].RawProfile,
		}
	}
	return alloy
}

func (c *Component) Push(ctx context.Context, req *connect.Request[pushv1.PushRequest],
) (*connect.Response[pushv1.PushResponse], error) {
	appendables := c.getAppendables()

	// Create an errgroup with the timeout context
	g, ctx := errgroup.WithContext(ctx)

	// Start copying the request body to all pipes
	for i := range appendables {
		appendable := appendables[i].Appender()
		g.Go(func() error {
			var (
				errs error
				lb   = labels.NewBuilder(nil)
			)

			for idx := range req.Msg.Series {
				lb.Reset(nil)
				setLabelBuilderFromAPI(lb, req.Msg.Series[idx].Labels)
				// Ensure service_name label is set
				lbls := ensureServiceName(lb.Labels())
				err := appendable.Append(ctx, lbls, apiToAlloySamples(req.Msg.Series[idx].Samples))
				if err != nil {
					errs = errors.Join(
						errs,
						fmt.Errorf("unable to append series %s to appendable %d: %w", lb.Labels().String(), i, err),
					)
				}
			}
			return errs
		})
	}
	if err := g.Wait(); err != nil {
		level.Error(c.opts.Logger).Log("msg", "Failed to forward profiles requests", "err", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	level.Debug(c.opts.Logger).Log("msg", "Profiles successfully forwarded")
	return connect.NewResponse(&pushv1.PushResponse{}), nil
}

func (c *Component) getAppendables() []pyroscope.Appendable {
	c.mut.Lock()
	defer c.mut.Unlock()
	appendables := c.appendables
	return appendables
}

func (c *Component) handleIngest(w http.ResponseWriter, r *http.Request) {
	appendables := c.getAppendables()

	// Parse labels early
	var lbls labels.Labels
	if nameParam := r.URL.Query().Get("name"); nameParam != "" {
		ls, err := labelset.Parse(nameParam)
		if err != nil {
			level.Warn(c.opts.Logger).Log(
				"msg", "Failed to parse labels from name parameter",
				"name", nameParam,
				"err", err,
			)
			// Continue with empty labels instead of returning an error
		} else {
			var labelPairs []labels.Label
			for k, v := range ls.Labels() {
				labelPairs = append(labelPairs, labels.Label{Name: k, Value: v})
			}
			lbls = labels.New(labelPairs...)
		}
	}

	// Ensure service_name label is set
	lbls = ensureServiceName(lbls)

	// Read the entire body into memory
	// This matches how Append() handles profile data (as RawProfile),
	// but means the entire profile will be held in memory
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r.Body); err != nil {
		level.Error(c.opts.Logger).Log("msg", "Failed to read request body", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	g, ctx := errgroup.WithContext(r.Context())

	// Process each appendable with a new reader from the buffer
	for i, appendable := range appendables {
		g.Go(func() error {
			profile := &pyroscope.IncomingProfile{
				RawBody: buf.Bytes(),
				Headers: r.Header.Clone(),
				URL:     r.URL,
				Labels:  lbls,
			}

			if err := appendable.Appender().AppendIngest(ctx, profile); err != nil {
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

// ensureServiceName ensures that the service_name label is set
func ensureServiceName(lbls labels.Labels) labels.Labels {
	builder := labels.NewBuilder(lbls)
	originalName := lbls.Get(pyroscope.LabelName)

	if !lbls.Has(pyroscope.LabelServiceName) {
		builder.Set(pyroscope.LabelServiceName, originalName)
	} else {
		builder.Set("app_name", originalName)
	}

	return builder.Labels()
}
