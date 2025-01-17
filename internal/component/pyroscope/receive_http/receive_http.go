package receive_http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"sync"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/alloy/internal/component"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
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
	// Server is the configuration for the HTTP server.
	Server *fnet.ServerConfig `alloy:",squash"`

	// Join takes a discovert.Target and will add information that can be matched with incoming profiles.
	//
	// The matching is taking place using
	// __meta_docker_network_ip
	// __meta_kubernetes_pod_ip
	Join []discovery.Target

	// ForwardTo is a list of appendables to forward the received profiles to.
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
	ipLookup           map[netip.Addr]discovery.Target
	mut                sync.RWMutex
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

	// build new ip lookup, before acquiring lock
	ipLookup := buildIPLookupMap(c.opts.Logger, newArgs.Join)

	c.mut.Lock()
	defer c.mut.Unlock()

	c.ipLookup = ipLookup
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

	// lookup extra labels to join
	var extraLabels discovery.Target
	if remoteIP := c.ipFromReq(req.Peer().Addr, req.Header()); remoteIP.IsValid() {
		extraLabels = c.getIPLookup()[remoteIP]
	}

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
				for k, v := range extraLabels {
					lb.Set(k, v)
				}
				err := appendable.Append(ctx, lb.Labels(), apiToAlloySamples(req.Msg.Series[idx].Samples))
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

func (c *Component) getIPLookup() map[netip.Addr]discovery.Target {
	c.mut.RLock()
	defer c.mut.RUnlock()
	ipLookup := c.ipLookup
	return ipLookup
}

func (c *Component) getAppendables() []pyroscope.Appendable {
	c.mut.RLock()
	defer c.mut.RUnlock()
	appendables := c.appendables
	return appendables
}

// TODO: This is likely to simple we should also accept headers X-Forwarded-For, if it coming from an internal IP
func (c *Component) ipFromReq(remoteAddr string, _ http.Header) netip.Addr {
	if remoteAddr != "" {
		host, _, _ := net.SplitHostPort(remoteAddr)
		addr, err := netip.ParseAddr(host)
		if err == nil {
			return addr
		}
		level.Error(c.opts.Logger).Log("msg", "Unable to parse remote IP address", "ip", host)
	}

	return netip.Addr{}
}

// Parse and rewrite labels/Series
// TODO: Keep labels out of url until pyroscope.write
// TODO: Investigate merging of appendables[0].Append() and AppendIngest again
func (c *Component) rewriteIngestURL(ip netip.Addr, u url.URL) url.URL {
	ipLookup := c.getIPLookup()

	// loop up ip in ipLookup
	extraLabels, found := ipLookup[ip]
	if !found {
		return u
	}

	// parse existing labels
	ls, err := labelset.Parse(u.Query().Get("name"))
	if err != nil {
		level.Warn(c.opts.Logger).Log("msg", "Failed to parse labelset", "err", err)
		return u
	}

	for k, v := range extraLabels {
		ls.Add(k, v)
	}
	query := u.Query()
	query.Set("name", ls.Normalized())
	u.RawQuery = query.Encode()

	return u
}

func (c *Component) handleIngest(w http.ResponseWriter, r *http.Request) {
	appendables := c.getAppendables()

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

	newURL := *r.URL
	remoteIP := c.ipFromReq(r.RemoteAddr, r.Header)
	if remoteIP.IsValid() {
		newURL = c.rewriteIngestURL(remoteIP, *r.URL)
	}

	// Process each appendable
	for i, appendable := range appendables {
		g.Go(func() error {
			defer pipeReaders[i].(io.ReadCloser).Close()

			profile := &pyroscope.IncomingProfile{
				Body:    io.NopCloser(pipeReaders[i]),
				Headers: r.Header.Clone(),
				URL:     &newURL,
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
