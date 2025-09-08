package write

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/util"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
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
)

var (
	userAgent        = useragent.Get()
	DefaultArguments = func() Arguments {
		return Arguments{
			Tracing: TracingOptions{
				JaegerPropagator:       true,
				TraceContextPropagator: true,
				HttpClientTraceErrors:  false,
				HttpClientTraceAll:     false,
			},
		}
	}
	_ component.Component = (*Component)(nil)
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.write",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			tracer := o.Tracer.Tracer("pyroscope.write")
			args := c.(Arguments)
			return New(
				o.Logger,
				tracer,
				o.Registerer,
				func(exports Exports) {
					o.OnStateChange(exports)
				},
				args,
			)
		},
	})
}

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
	HttpClientTraceErrors  bool `alloy:"http_client_trace_errors,attr,optional"`
	HttpClientTraceAll     bool `alloy:"http_client_trace_all,attr,optional"`
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
	arguments     Arguments
	metrics       *metrics
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
	c Arguments,
) (*Component, error) {

	metrics := newMetrics(reg)
	receiver, err := newFanOut(logger, tracer, c, metrics)
	if err != nil {
		return nil, err
	}
	// Immediately export the receiver
	onStateChange(Exports{Receiver: receiver})

	return &Component{
		arguments:     c,
		logger:        logger,
		tracer:        tracer,
		onStateChange: onStateChange,
		metrics:       metrics,
	}, nil
}

var _ component.Component = (*Component)(nil)

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// Update implements Component.
func (c *Component) Update(newConfig component.Arguments) error {
	c.arguments = newConfig.(Arguments)
	receiver, err := newFanOut(c.logger, c.tracer, newConfig.(Arguments), c.metrics)
	if err != nil {
		return err
	}
	c.onStateChange(Exports{Receiver: receiver})
	return nil
}

type fanOutClient struct {
	endpoints []endpoint
	arguments Arguments
	metrics   *metrics
	tracer    trace.Tracer
	logger    log.Logger
}

type endpoint struct {
	pushClient   pushv1connect.PusherServiceClient
	ingestClient *http.Client
	options      *EndpointOptions
	url          *url.URL
}

// newFanOut creates a new fan out client that will fan out to all endpoints.
func newFanOut(logger log.Logger, tracer trace.Tracer, config Arguments, metrics *metrics) (*fanOutClient, error) {
	clients := make([]endpoint, 0, len(config.Endpoints))
	uid := alloyseed.Get().UID

	for _, e := range config.Endpoints {
		u, err := url.Parse(e.URL)
		if err != nil {
			return nil, fmt.Errorf("parse URL: %w", err)
		}
		if e.Headers == nil {
			e.Headers = map[string]string{}
		}
		e.Headers[alloyseed.LegacyHeaderName] = uid
		e.Headers[alloyseed.HeaderName] = uid
		e.Headers["User-Agent"] = userAgent
		httpClient, err := commonconfig.NewClientFromConfig(*e.HTTPClientConfig.Convert(), e.Name)
		if err != nil {
			return nil, err
		}
		configureTracing(config, httpClient)

		push := pushv1connect.NewPusherServiceClient(httpClient, e.URL)
		clients = append(
			clients,
			endpoint{
				pushClient:   push,
				ingestClient: httpClient,
				options:      e,
				url:          u,
			},
		)
	}
	return &fanOutClient{
		endpoints: clients,
		logger:    logger,
		tracer:    tracer,
		arguments: config,
		metrics:   metrics,
	}, nil
}

type forwardRequest struct {
	op                    string
	reqSize, profileCount int64
	impl                  func(ctx context.Context, l log.Logger, e endpoint) error
}

// forward forwards and multiplex push and ingest requests down to endpoints with retries
func (f *fanOutClient) forward(ctx context.Context, req forwardRequest) error {
	defer f.observeLatency("-", req.op+"_total")()

	ctx, sp := f.tracer.Start(ctx, req.op)
	defer sp.End()

	var (
		wg       sync.WaitGroup
		errs     error
		errorMut sync.Mutex
		dl       any
		ok       bool
		l        = util.TraceLog(f.logger, sp)
		st       = time.Now()
	)
	l = log.With(l, "op", req.op)
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
			"sz", req.reqSize,
			"n", req.profileCount,
			"dl", dl,
			"st", st,
		)
	}()

	for _, e := range f.endpoints {
		wg.Add(1)
		go func() {
			var (
				b = backoff.New(ctx, backoff.Config{
					MinBackoff: e.options.MinBackoff,
					MaxBackoff: e.options.MaxBackoff,
					MaxRetries: e.options.MaxBackoffRetries,
				})
				err error
			)
			defer f.observeLatency(e.options.URL, req.op+"_endpoint")()
			defer wg.Done()

			for {
				err = f.forwardDownstream(ctx, l, e, req)
				if err == nil {
					f.metrics.sentBytes.WithLabelValues(e.options.URL).Add(float64(req.reqSize))
					f.metrics.sentProfiles.WithLabelValues(e.options.URL).Add(float64(req.profileCount))
					break
				}
				_ = level.Debug(l).Log(
					"msg", "failed to forward to endpoint",
					"endpoint", e.options.URL,
					"retries", b.NumRetries(),
					"err", err,
				)
				if !shouldRetry(err) {
					break
				}
				b.Wait()
				if !b.Ongoing() {
					break
				}
				f.metrics.retries.WithLabelValues(e.options.URL).Inc()
			}
			if err != nil {
				f.metrics.droppedBytes.WithLabelValues(e.options.URL).Add(float64(req.reqSize))
				f.metrics.droppedProfiles.WithLabelValues(e.options.URL).Add(float64(req.profileCount))
				err = fmt.Errorf("failed to forward to endpoint %s (%d retries): %w", e.options.URL, b.NumRetries(), err)
				util.ErrorsJoinConcurrent(&errs, err, &errorMut)
			}
		}()
	}

	wg.Wait()
	return errs
}

// Push implements the PusherServiceClient interface.
func (f *fanOutClient) Push(
	ctx context.Context,
	req *connect.Request[pushv1.PushRequest],
) (*connect.Response[pushv1.PushResponse], error) {
	reqSize, profileCount := requestSize(req)
	err := f.forward(ctx, forwardRequest{
		op:           "push",
		reqSize:      reqSize,
		profileCount: profileCount,
		impl: func(ctx context.Context, l log.Logger, e endpoint) error {
			return f.pushDownstream(ctx, e, req.Msg)
		},
	})
	if err != nil {
		return nil, err
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
		protoLabels  = make([]*typesv1.LabelPair, 0, len(lbs)+len(f.arguments.ExternalLabels))
		protoSamples = make([]*pushv1.RawSample, 0, len(samples))
		lbsBuilder   = labels.NewBuilder(nil)
	)

	for _, label := range lbs {
		// filter reserved labels, with exceptions for __name__ and __delta__.
		if strings.HasPrefix(label.Name, model.ReservedLabelPrefix) &&
			label.Name != labels.MetricName &&
			label.Name != pyroscope.LabelNameDelta {

			continue
		}
		lbsBuilder.Set(label.Name, label.Value)
	}
	for name, value := range f.arguments.ExternalLabels {
		lbsBuilder.Set(name, value)
	}
	for _, l := range lbsBuilder.Labels() {
		protoLabels = append(protoLabels, &typesv1.LabelPair{
			Name:  l.Name,
			Value: l.Value,
		})
	}
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
	for k, v := range f.arguments.ExternalLabels {
		ls.Add(k, v)
	}
	query.Set("name", ls.Normalized())

	return f.forward(ctx, forwardRequest{
		op:           "ingest",
		reqSize:      int64(len(profile.RawBody)),
		profileCount: 1,
		impl: func(ctx context.Context, l log.Logger, e endpoint) error {
			return f.ingestDownstream(ctx, e, profile, query)
		},
	})
}

func (f *fanOutClient) observeLatency(endpoint, latencyType string) func() {
	t := time.Now()
	return func() {
		f.metrics.latency.WithLabelValues(endpoint, latencyType).Observe(time.Since(t).Seconds())
	}
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

	sort.Sort(lbls)

	lastLabelName := ""
	for _, l := range lbls {
		if cmp := strings.Compare(lastLabelName, l.Name); cmp == 0 {
			return fmt.Errorf("duplicate label name: %s", l.Name)
		}

		// Validate label value
		if !model.LabelValue(l.Value).IsValid() {
			return fmt.Errorf("invalid label value for %s: %s", l.Name, l.Value)
		}

		// Skip label name validation for pyroscope reserved labels
		if l.Name != pyroscope.LabelName {
			// Validate label name
			if err := labelset.ValidateLabelName(l.Name); err != nil {
				return fmt.Errorf("invalid label name: %w", err)
			}
		}

		lastLabelName = l.Name
	}

	return nil
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

func (f *fanOutClient) forwardDownstream(ctx context.Context, l log.Logger, e endpoint, req forwardRequest) error {
	defer f.observeLatency(e.options.URL, req.op+"_downstream")()
	downstreamContext, cancel := context.WithTimeout(ctx, e.options.RemoteTimeout)
	defer cancel()

	if !f.arguments.Tracing.HttpClientTraceAll && !f.arguments.Tracing.HttpClientTraceErrors {
		return req.impl(downstreamContext, l, e)
	}
	ct := newClientTrace()
	ctx = httptrace.WithClientTrace(ctx, ct.trace)
	err := req.impl(ctx, l, e)
	if f.arguments.Tracing.HttpClientTraceAll || (f.arguments.Tracing.HttpClientTraceErrors && err != nil) {
		ct.flush(level.Debug(l))
	}
	return err
}

func (f *fanOutClient) pushDownstream(ctx context.Context, e endpoint, msg *pushv1.PushRequest) error {
	req := connect.NewRequest(msg)
	for k, v := range e.options.Headers {
		req.Header().Set(k, v)
	}
	_, err := e.pushClient.Push(ctx, req)
	return err
}

func (f *fanOutClient) ingestDownstream(ctx context.Context, e endpoint, profile *pyroscope.IncomingProfile, query url.Values) error {
	u := *e.url
	u.Path = path.Join(u.Path, profile.URL.Path)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(profile.RawBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	for k, v := range e.options.Headers {
		req.Header.Set(k, v)
	}

	for idx := range profile.ContentType {
		if idx == 0 {
			req.Header.Set(pyroscope.HeaderContentType, profile.ContentType[idx])
			continue
		}
		req.Header.Add(pyroscope.HeaderContentType, profile.ContentType[idx])
	}

	resp, err := e.ingestClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		wErr := &PyroscopeWriteError{StatusCode: resp.StatusCode}
		wErr.readBody(resp)
		return fmt.Errorf("remote error: %w", wErr)
	}

	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	return nil
}
