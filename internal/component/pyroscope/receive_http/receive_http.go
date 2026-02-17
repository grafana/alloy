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
	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/pyroscope"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.receive_http",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			tracer := opts.Tracer.Tracer("pyroscope.receive_http")
			return New(opts.Logger, tracer, opts.Registerer, args.(Arguments))
		},
	})
}

type Arguments struct {
	Server    *fnet.ServerConfig     `alloy:",squash"`
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server: fnet.DefaultServerConfig(),
	}
	a.Server.HTTP.ConnLimit = 64 / 4 * 1024
}

type Component struct {
	server             *fnet.TargetServer
	serverConfig       *fnet.HTTPConfig
	uncheckedCollector *util.UncheckedCollector
	appendables        []pyroscope.Appendable
	grpcServer         *grpc.Server
	mut                sync.Mutex
	logger             log.Logger
	tracer             trace.Tracer
}

func New(logger log.Logger, tracer trace.Tracer, reg prometheus.Registerer, args Arguments) (*Component, error) {
	uncheckedCollector := util.NewUncheckedCollector(nil)
	reg.MustRegister(uncheckedCollector)

	c := &Component{
		logger:             logger,
		tracer:             tracer,
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
	level.Info(c.logger).Log("msg", "terminating due to context done")
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	_, err := c.update(args)
	return err
}

// returns true if the server was shutdown
func (c *Component) update(args component.Arguments) (bool, error) {
	shutdown := false
	newArgs := args.(Arguments)
	// required for debug info upload over grpc over http2 over http server port
	if newArgs.Server.HTTP.HTTP2 == nil {
		newArgs.Server.HTTP.HTTP2 = &fnet.HTTP2Config{}
	}
	newArgs.Server.HTTP.HTTP2.Enabled = true

	c.mut.Lock()
	defer c.mut.Unlock()

	c.appendables = newArgs.ForwardTo

	serverNeedsRestarting := !reflect.DeepEqual(c.serverConfig, newArgs.Server.HTTP)
	if !serverNeedsRestarting {
		return shutdown, nil
	}
	shutdown = true
	c.shutdownServer()
	c.server = nil
	c.serverConfig = nil

	// [server.Server] registers new metrics every time it is created. To
	// avoid issues with re-registering metrics with the same name, we create a
	// new registry for the server every time we create one, and pass it to an
	// unchecked collector to bypass uniqueness checking.
	serverRegistry := prometheus.NewRegistry()
	c.uncheckedCollector.SetCollector(serverRegistry)

	srv, err := fnet.NewTargetServer(c.logger, "pyroscope_receive_http", serverRegistry, newArgs.Server)
	if err != nil {
		return shutdown, fmt.Errorf("failed to create server: %w", err)
	}
	c.server = srv
	c.serverConfig = newArgs.Server.HTTP

	return shutdown, c.server.MountAndRun(func(router *mux.Router) {
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
			ID:         api[i].ID,
			RawProfile: api[i].RawProfile,
		}
	}
	return alloy
}

func (c *Component) Push(ctx context.Context, req *connect.Request[pushv1.PushRequest],
) (*connect.Response[pushv1.PushResponse], error) {

	appendables := c.getAppendables()

	ctx, sp := c.tracer.Start(ctx, "/push.v1.PusherService/Push")
	defer sp.End()
	l := pyroutil.TraceLog(c.logger, sp)

	var wg sync.WaitGroup
	var errs error
	var errorMut sync.Mutex

	// Start copying the request body to all pipes
	for i := range appendables {
		appendable := appendables[i].Appender()
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lb = labels.NewBuilder(labels.EmptyLabels())

			for idx := range req.Msg.Series {
				lb.Reset(labels.EmptyLabels())
				setLabelBuilderFromAPI(lb, req.Msg.Series[idx].Labels)
				// Ensure service_name label is set
				lbls := ensureServiceName(lb.Labels())
				err := appendable.Append(ctx, lbls, apiToAlloySamples(req.Msg.Series[idx].Samples))
				if err != nil {
					pyroutil.ErrorsJoinConcurrent(
						&errs,
						fmt.Errorf("unable to append series %s to appendable %d: %w", lb.Labels().String(), i, err),
						&errorMut,
					)
				}
			}
		}()
	}
	wg.Wait()
	if errs != nil {
		level.Warn(l).Log("msg", "Failed to forward profiles requests", "err", errs)
		return nil, connect.NewError(connect.CodeInternal, errs)
	}

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

	ctx := r.Context()

	ctx, sp := c.tracer.Start(ctx, "/ingest")
	defer sp.End()

	l := pyroutil.TraceLog(c.logger, sp)

	// Parse labels early
	var lbls labels.Labels
	if nameParam := r.URL.Query().Get("name"); nameParam != "" {
		ls, err := labelset.Parse(nameParam)
		if err != nil {
			level.Warn(l).Log(
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
	} // todo this is a required parameter, treat absence as error

	// Ensure service_name label is set
	lbls = ensureServiceName(lbls)

	// Read the entire body into memory
	// This matches how Append() handles profile data (as RawProfile),
	// but means the entire profile will be held in memory
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r.Body); err != nil {
		level.Warn(l).Log("msg", "Failed to read request body", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	var errs error
	var errorMut sync.Mutex

	// Process each appendable with a new reader from the buffer]
	for i, appendable := range appendables {
		wg.Add(1)
		go func() {
			defer wg.Done()
			profile := &pyroscope.IncomingProfile{
				RawBody:     buf.Bytes(),
				ContentType: r.Header.Values(pyroscope.HeaderContentType),
				URL:         r.URL,
				Labels:      lbls,
			}

			if err := appendable.Appender().AppendIngest(ctx, profile); err != nil {
				err = fmt.Errorf("failed to ingest profile to appendable %d: %w", i, err)
				pyroutil.ErrorsJoinConcurrent(&errs, err, &errorMut)
			}
		}()
	}

	wg.Wait()

	if errs != nil {
		level.Warn(l).Log("msg", "Failed to ingest profiles", "err", errs)
		var writeErr *write.PyroscopeWriteError
		if errors.As(errs, &writeErr) {
			http.Error(w, http.StatusText(writeErr.StatusCode), writeErr.StatusCode)
		} else {
			http.Error(w, "Failed to process request", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Component) shutdownServer() {
	if c.grpcServer != nil {
		c.grpcServer.GracefulStop()
		c.grpcServer = nil
	}
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
