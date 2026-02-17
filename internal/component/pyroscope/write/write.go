package write

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"connectrpc.com/connect"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/util"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"github.com/grafana/dskit/backoff"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/prometheus/client_golang/prometheus"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

var (
	DefaultArguments = func() Arguments {
		return Arguments{
			Tracing: TracingOptions{
				JaegerPropagator:       true,
				TraceContextPropagator: true,
			},
		}
	}
)

// Arguments represents the input state of the pyroscope.write
// component.
type Arguments struct {
	ExternalLabels map[string]string  `alloy:"external_labels,attr,optional"`
	Endpoints      []*EndpointOptions `alloy:"endpoint,block,optional"`
	Tracing        TracingOptions     `alloy:"tracing,block,optional"`
}

type TracingOptions struct {
	JaegerPropagator       bool `alloy:"jaeger_propagator,attr,optional"`
	TraceContextPropagator bool `alloy:"trace_context_propagator,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (rc *Arguments) SetToDefault() {
	*rc = DefaultArguments()
}

// EndpointOptions describes an individual location for where profiles
// should be delivered to using the Pyroscope push API.
type EndpointOptions struct {
	Name              string                   `alloy:"name,attr,optional"`
	URL               string                   `alloy:"url,attr"`
	RemoteTimeout     time.Duration            `alloy:"remote_timeout,attr,optional"`
	Headers           map[string]string        `alloy:"headers,attr,optional"`
	HTTPClientConfig  *config.HTTPClientConfig `alloy:",squash"`
	MinBackoff        time.Duration            `alloy:"min_backoff_period,attr,optional"`  // start backoff at this level
	MaxBackoff        time.Duration            `alloy:"max_backoff_period,attr,optional"`  // increase exponentially to this level
	MaxBackoffRetries int                      `alloy:"max_backoff_retries,attr,optional"` // give up after this many; zero means infinite retries
}

func GetDefaultEndpointOptions() EndpointOptions {
	defaultEndpointOptions := EndpointOptions{
		RemoteTimeout:     10 * time.Second,
		MinBackoff:        500 * time.Millisecond,
		MaxBackoff:        5 * time.Minute,
		MaxBackoffRetries: 10,
		HTTPClientConfig:  config.CloneDefaultHTTPClientConfig(),
	}

	return defaultEndpointOptions
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointOptions) SetToDefault() {
	*r = GetDefaultEndpointOptions()
}

// Validate implements syntax.Validator.
func (r *EndpointOptions) Validate() error {
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	if r.HTTPClientConfig != nil {
		return r.HTTPClientConfig.Validate()
	}

	return nil
}

// Component is the pyroscope.write component.
type Component struct {
	logger        log.Logger
	tracer        trace.Tracer
	onStateChange func(Exports)
	cfg           Arguments
	metrics       *metrics
	userAgent     string
	uid           string
	dataPath      string

	mu             sync.Mutex
	receiver       *fanOutClient
	runCtx         context.Context
	receiverCancel context.CancelFunc
}

// Exports are the set of fields exposed by the pyroscope.write component.
type Exports struct {
	Receiver pyroscope.Appendable `alloy:"receiver,attr"`
}

// New creates a new pyroscope.write component.
func New(
	logger log.Logger,
	tracer trace.Tracer,
	reg prometheus.Registerer,
	onStateChange func(Exports),
	userAgent, uid string,
	dataPath string,
	c Arguments,
) (*Component, error) {

	m := newMetrics(reg)
	receiver, err := newFanOut(logger, tracer, c, m, userAgent, uid, dataPath)
	if err != nil {
		return nil, err
	}
	// Immediately export the receiver
	onStateChange(Exports{Receiver: receiver})

	return &Component{
		cfg:           c,
		logger:        logger,
		tracer:        tracer,
		onStateChange: onStateChange,
		metrics:       m,
		userAgent:     userAgent,
		uid:           uid,
		dataPath:      dataPath,
		receiver:      receiver,
	}, nil
}

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	var receiverCtx context.Context

	c.mu.Lock()
	c.runCtx = ctx
	receiverCtx, c.receiverCancel = context.WithCancel(ctx)
	c.receiver.Start(receiverCtx)
	c.mu.Unlock()

	<-ctx.Done()

	c.mu.Lock()
	c.receiverCancel()
	c.receiver.Wait()
	c.runCtx = nil
	c.receiverCancel = nil
	c.mu.Unlock()

	return ctx.Err()
}

// Update implements Component.
func (c *Component) Update(newConfig Arguments) error {
	c.cfg = newConfig
	receiver, err := newFanOut(c.logger, c.tracer, newConfig, c.metrics, c.userAgent, c.uid, c.dataPath)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.receiverCancel != nil {
		c.receiverCancel()
		c.receiver.Wait()
	}

	c.receiver = receiver
	c.onStateChange(Exports{Receiver: receiver})

	if c.runCtx != nil {
		var receiverCtx context.Context
		receiverCtx, c.receiverCancel = context.WithCancel(c.runCtx)
		c.receiver.Start(receiverCtx)
	}

	return nil
}

type fanOutClient struct {
	// The list of push clients to fan out to.
	pushClients []pushv1connect.PusherServiceClient

	debugInfos []*debuginfo.Client

	ingestClients map[*EndpointOptions]*http.Client
	config        Arguments
	metrics       *metrics
	tracer        trace.Tracer
	logger        log.Logger

	uploaderWg sync.WaitGroup
}

func (f *fanOutClient) Client() debuginfogrpc.DebuginfoServiceClient {
	for _, client := range f.debugInfos {
		cl := client.Client()
		if cl != nil {
			return cl
		}
	}
	return nil
}

func (f *fanOutClient) Upload(j debuginfo.UploadJob) {
	for _, u := range f.debugInfos {
		u.Upload(j)
	}
}

func (f *fanOutClient) Start(ctx context.Context) {
	for _, u := range f.debugInfos {
		f.uploaderWg.Add(1)
		go func(c *debuginfo.Client) {
			defer f.uploaderWg.Done()
			if err := c.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				level.Error(f.logger).Log("msg", "debuginfo uploader error", "err", err)
			}
		}(u)
	}
}

func (f *fanOutClient) Wait() {
	f.uploaderWg.Wait()
}

// newFanOut creates a new fan out client that will fan out to all endpoints.
func newFanOut(logger log.Logger, tracer trace.Tracer, config Arguments, metrics *metrics, userAgent string, uid string, dataPath string) (*fanOutClient, error) {
	pushClients := make([]pushv1connect.PusherServiceClient, 0, len(config.Endpoints))
	debugInfos := make([]*debuginfo.Client, 0, len(config.Endpoints))
	ingestClients := make(map[*EndpointOptions]*http.Client)

	for i, endpoint := range config.Endpoints {
		u, err := url.Parse(endpoint.URL)
		if err != nil {
			return nil, err
		}
		if endpoint.Headers == nil {
			endpoint.Headers = map[string]string{}
		}
		endpoint.Headers["X-Alloy-Id"] = uid
		httpClient, err := commonconfig.NewClientFromConfig(*endpoint.HTTPClientConfig.Convert(), endpoint.Name)
		if err != nil {
			return nil, err
		}
		configureTracing(config, httpClient)

		pushClients = append(
			pushClients,
			pushv1connect.NewPusherServiceClient(httpClient, endpoint.URL, WithUserAgent(userAgent)),
		)
		ingestClients[endpoint] = httpClient

		endpointDataPath := filepath.Join(dataPath, fmt.Sprintf("endpoint-%d", i))
		debugInfo := debuginfo.NewClient(logger, func() (*grpc.ClientConn, error) {
			return newDebugInfoGRPCClient(u, endpoint)
		}, metrics.debugInfoUploadBytes, endpointDataPath)
		debugInfos = append(debugInfos, debugInfo)
	}

	return &fanOutClient{
		logger:        logger,
		tracer:        tracer,
		pushClients:   pushClients,
		debugInfos:    debugInfos,
		ingestClients: ingestClients,
		config:        config,
		metrics:       metrics,
	}, nil
}

// Push implements the PusherServiceClient interface.
func (f *fanOutClient) Push(
	ctx context.Context,
	req *connect.Request[pushv1.PushRequest],
) (*connect.Response[pushv1.PushResponse], error) {

	defer f.observeLatency("-", "push_total")()

	ctx, sp := f.tracer.Start(ctx, "Push")
	defer sp.End()

	var (
		wg                    sync.WaitGroup
		errs                  error
		errorMut              sync.Mutex
		dl                    any
		ok                    bool
		reqSize, profileCount = requestSize(req)
		l                     = util.TraceLog(f.logger, sp)
		st                    = time.Now()
	)
	if dl, ok = ctx.Deadline(); !ok {
		dl = "none"
	}
	defer func() {
		if errs != nil {
			l = level.Warn(log.With(l, "err", errs))
		} else {
			l = level.Debug(l)
		}
		_ = l.Log(
			"msg", "Push",
			"sz", reqSize,
			"n", profileCount,
			"dl", dl,
			"st", st,
		)
	}()

	for i, client := range f.pushClients {
		var (
			client  = client
			i       = i
			backoff = backoff.New(ctx, backoff.Config{
				MinBackoff: f.config.Endpoints[i].MinBackoff,
				MaxBackoff: f.config.Endpoints[i].MaxBackoff,
				MaxRetries: f.config.Endpoints[i].MaxBackoffRetries,
			})
			err error
		)
		wg.Add(1)
		go func() {
			defer f.observeLatency(f.config.Endpoints[i].URL, "push_endpoint")()
			defer wg.Done()
			req := connect.NewRequest(req.Msg)
			for k, v := range f.config.Endpoints[i].Headers {
				req.Header().Set(k, v)
			}
			for {
				err = func() error {
					defer f.observeLatency(f.config.Endpoints[i].URL, "push_downstream")()
					ctx, cancel := context.WithTimeout(ctx, f.config.Endpoints[i].RemoteTimeout)
					defer cancel()

					_, err := client.Push(ctx, req)
					return err
				}()
				if err == nil {
					f.metrics.sentBytes.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(reqSize))
					f.metrics.sentProfiles.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(profileCount))
					break
				}
				_ = level.Debug(l).Log("msg",
					"failed to push to endpoint",
					"endpoint", f.config.Endpoints[i].URL,
					"retries", backoff.NumRetries(),
					"err", err,
				)
				if !shouldRetry(err) {
					break
				}
				backoff.Wait()
				if !backoff.Ongoing() {
					break
				}
				f.metrics.retries.WithLabelValues(f.config.Endpoints[i].URL).Inc()
			}
			if err != nil {
				f.metrics.droppedBytes.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(reqSize))
				f.metrics.droppedProfiles.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(profileCount))
				err = fmt.Errorf("failed to push to endpoint %s (%d retries): %w", f.config.Endpoints[i].URL, backoff.NumRetries(), err)
				util.ErrorsJoinConcurrent(&errs, err, &errorMut)
			}
		}()
	}

	wg.Wait()
	if errs != nil {
		return nil, errs
	}
	return connect.NewResponse(&pushv1.PushResponse{}), nil
}

func shouldRetry(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var writeErr *PyroscopeWriteError
	if errors.As(err, &writeErr) {
		status := writeErr.StatusCode
		if status == http.StatusTooManyRequests || status == http.StatusRequestTimeout {
			return true
		}
		return status >= http.StatusInternalServerError
	}

	switch connect.CodeOf(err) {
	case connect.CodeDeadlineExceeded, connect.CodeUnknown,
		connect.CodeResourceExhausted, connect.CodeInternal,
		connect.CodeUnavailable, connect.CodeDataLoss, connect.CodeAborted:
		return true
	}
	return false
}

func requestSize(req *connect.Request[pushv1.PushRequest]) (int64, int64) {
	var size, profiles int64
	for _, raw := range req.Msg.Series {
		for _, sample := range raw.Samples {
			size += int64(len(sample.RawProfile))
			profiles++
		}
	}
	return size, profiles
}

// Appender implements the pyroscope.Appendable interface.
func (f *fanOutClient) Appender() pyroscope.Appender {
	return f
}

// Append implements the Appender interface.
func (f *fanOutClient) Append(ctx context.Context, lbs labels.Labels, samples []*pyroscope.RawSample) error {
	// Validate labels first
	if err := validateLabels(lbs); err != nil {
		return fmt.Errorf("invalid labels in profile: %w", err)
	}

	// todo(ctovena): we should probably pool the label pair arrays and label builder to avoid allocs.
	var (
		protoLabels  = make([]*typesv1.LabelPair, 0, lbs.Len()+len(f.config.ExternalLabels))
		protoSamples = make([]*pushv1.RawSample, 0, len(samples))
		lbsBuilder   = labels.NewBuilder(labels.EmptyLabels())
	)

	lbs.Range(func(label labels.Label) {
		// filter reserved labels, with exceptions for __name__ and __delta__.
		if strings.HasPrefix(label.Name, model.ReservedLabelPrefix) &&
			label.Name != model.MetricNameLabel &&
			label.Name != pyroscope.LabelNameDelta {

			return
		}
		lbsBuilder.Set(label.Name, label.Value)
	})
	for name, value := range f.config.ExternalLabels {
		lbsBuilder.Set(name, value)
	}
	lbsBuilder.Labels().Range(func(l labels.Label) {
		protoLabels = append(protoLabels, &typesv1.LabelPair{
			Name:  l.Name,
			Value: l.Value,
		})
	})
	for _, sample := range samples {
		protoSamples = append(protoSamples, &pushv1.RawSample{
			ID:         sample.ID,
			RawProfile: sample.RawProfile,
		})
	}
	// push to all clients
	_, err := f.Push(ctx, connect.NewRequest(&pushv1.PushRequest{
		Series: []*pushv1.RawProfileSeries{
			{Labels: protoLabels, Samples: protoSamples},
		},
	}))
	return err
}

type PyroscopeWriteError struct {
	Message    string
	StatusCode int
}

func (e *PyroscopeWriteError) Error() string {
	return fmt.Sprintf("pyroscope write error: status=%d msg=%s", e.StatusCode, e.Message)
}

func (e *PyroscopeWriteError) readBody(resp *http.Response) {
	// read the first 2028 bytes of error body
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err == nil {
		e.Message = string(body)
	}

	// ensure full body is read to keep http connection Keep-Alive
	_, _ = io.Copy(io.Discard, resp.Body)
}

// AppendIngest implements the pyroscope.Appender interface.
func (f *fanOutClient) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	defer f.observeLatency("-", "ingest_total")()

	ctx, sp := f.tracer.Start(ctx, "AppendIngest")
	defer sp.End()

	var (
		wg                    sync.WaitGroup
		errs                  error
		errorMut              sync.Mutex
		dl                    any
		ok                    bool
		reqSize, profileCount = int64(len(profile.RawBody)), int64(1)
		l                     = util.TraceLog(f.logger, sp)
		st                    = time.Now()
	)
	if dl, ok = ctx.Deadline(); !ok {
		dl = "none"
	}
	defer func() {
		if errs != nil {
			l = level.Warn(log.With(l, "err", errs))
		} else {
			l = level.Debug(l)
		}
		_ = l.Log(
			"msg", "AppendIngest",
			"sz", reqSize,
			"n", profileCount,
			"dl", dl,
			"st", st,
		)
	}()

	// Handle labels
	query := profile.URL.Query()
	ls := labelset.New(make(map[string]string))

	finalLabels := ensureNameMatchesService(profile.Labels)

	if err := validateLabels(finalLabels); err != nil {
		return fmt.Errorf("invalid labels in profile: %w", err)
	}

	finalLabels.Range(func(l labels.Label) {
		ls.Add(l.Name, l.Value)
	})

	// Add external labels (which will override any existing ones)
	for k, v := range f.config.ExternalLabels {
		ls.Add(k, v)
	}
	query.Set("name", ls.Normalized())

	// Send to each endpoint concurrently
	for endpointIdx, endpoint := range f.config.Endpoints {
		var (
			endpoint = endpoint
			i        = endpointIdx
			backoff  = backoff.New(ctx, backoff.Config{
				MinBackoff: f.config.Endpoints[i].MinBackoff,
				MaxBackoff: f.config.Endpoints[i].MaxBackoff,
				MaxRetries: f.config.Endpoints[i].MaxBackoffRetries,
			})
			err error
		)
		wg.Add(1)
		go func() {
			defer f.observeLatency(endpoint.URL, "ingest_endpoint")()
			defer wg.Done()
			for {
				err = func() error {
					defer f.observeLatency(endpoint.URL, "ingest_downstream")()
					u, err := url.Parse(endpoint.URL)
					if err != nil {
						return fmt.Errorf("parse URL: %w", err)
					}

					u.Path = path.Join(u.Path, profile.URL.Path)

					// attach labels
					u.RawQuery = query.Encode()

					ctx, cancel := context.WithTimeout(ctx, f.config.Endpoints[i].RemoteTimeout)
					defer cancel()

					req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(profile.RawBody))
					if err != nil {
						return fmt.Errorf("create request: %w", err)
					}

					// set headers from endpoint
					for k, v := range endpoint.Headers {
						req.Header.Set(k, v)
					}

					// now set profile content type, overwrite what existed
					for idx := range profile.ContentType {
						if idx == 0 {
							req.Header.Set(pyroscope.HeaderContentType, profile.ContentType[idx])
							continue
						}
						req.Header.Add(pyroscope.HeaderContentType, profile.ContentType[idx])
					}

					resp, err := f.ingestClients[endpoint].Do(req)
					if err != nil {
						return fmt.Errorf("do request: %w", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						wErr := &PyroscopeWriteError{StatusCode: resp.StatusCode}
						wErr.readBody(resp)
						return fmt.Errorf("remote error: %w", wErr)
					}

					// Ensure full body is read to keep http connection Keep-Alive
					_, err = io.Copy(io.Discard, resp.Body)
					if err != nil {
						return fmt.Errorf("reading response body: %w", err)
					}

					return nil
				}()
				if err == nil {
					f.metrics.sentBytes.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(reqSize))
					f.metrics.sentProfiles.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(profileCount))
					break
				}
				_ = level.Debug(l).Log(
					"msg", "failed to ingest to endpoint",
					"endpoint", f.config.Endpoints[i].URL,
					"retries", backoff.NumRetries(),
					"err", err)
				if !shouldRetry(err) {
					break
				}
				backoff.Wait()
				if !backoff.Ongoing() {
					break
				}
				f.metrics.retries.WithLabelValues(f.config.Endpoints[i].URL).Inc()
			}
			if err != nil {
				f.metrics.droppedBytes.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(reqSize))
				f.metrics.droppedProfiles.WithLabelValues(f.config.Endpoints[i].URL).Add(float64(profileCount))
				err = fmt.Errorf("failed to ingest to endpoint %s (%d retries): %w", f.config.Endpoints[i].URL, backoff.NumRetries(), err)
				util.ErrorsJoinConcurrent(&errs, err, &errorMut)
			}
		}()
	}

	wg.Wait()

	return errs
}

func (f *fanOutClient) observeLatency(endpoint, latencyType string) func() {
	t := time.Now()
	return func() {
		f.metrics.latency.WithLabelValues(endpoint, latencyType).Observe(time.Since(t).Seconds())
	}
}

// WithUserAgent returns a `connect.ClientOption` that sets the User-Agent header on.
func WithUserAgent(agent string) connect.ClientOption {
	return connect.WithInterceptors(&agentInterceptor{agent})
}

type agentInterceptor struct {
	agent string
}

func (i *agentInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("User-Agent", i.agent)
		return next(ctx, req)
	}
}

func (i *agentInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("User-Agent", i.agent)
		return conn
	}
}

func (i *agentInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func ensureNameMatchesService(lbls labels.Labels) labels.Labels {
	if serviceName := lbls.Get(pyroscope.LabelServiceName); serviceName != "" {
		builder := labels.NewBuilder(lbls)
		builder.Set(pyroscope.LabelName, serviceName)
		return builder.Labels()
	}
	return lbls
}

// validateLabels checks for valid labels and doesn't contain duplicates.
func validateLabels(lbls labels.Labels) error {
	if lbls.Len() == 0 {
		return labelset.ErrServiceNameIsRequired
	}

	lastLabelName := ""
	var err error = nil
	lbls.Range(func(l labels.Label) {
		if err != nil {
			return // short-circuit so we return the first encountered error
		}

		if cmp := strings.Compare(lastLabelName, l.Name); cmp == 0 {
			err = fmt.Errorf("duplicate label name: %s", l.Name)
			return
		}

		// Validate label value
		if !model.LabelValue(l.Value).IsValid() {
			err = fmt.Errorf("invalid label value for %s: %s", l.Name, l.Value)
			return
		}

		// Skip label name validation for pyroscope reserved labels
		if l.Name != pyroscope.LabelName {
			// Validate label name
			if err = labelset.ValidateLabelName(l.Name); err != nil {
				err = fmt.Errorf("invalid label name: %w", err)
				return
			}
		}

		lastLabelName = l.Name
	})

	return err
}

func configureTracing(config Arguments, httpClient *http.Client) {
	if config.Tracing.JaegerPropagator || config.Tracing.TraceContextPropagator {
		var propagators []propagation.TextMapPropagator
		if config.Tracing.JaegerPropagator {
			propagators = append(propagators, jaeger.Jaeger{}) // pyroscope uses jaeger
		}
		if config.Tracing.TraceContextPropagator {
			propagators = append(propagators, propagation.TraceContext{}) // for good luck
		}
		httpClient.Transport = otelhttp.NewTransport(httpClient.Transport,
			otelhttp.WithPropagators(
				propagation.NewCompositeTextMapPropagator(propagators...),
			),
		)
	}
}
