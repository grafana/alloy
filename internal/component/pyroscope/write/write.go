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
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/oklog/run"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/dskit/backoff"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
)

var (
	userAgent        = useragent.Get()
	DefaultArguments = func() Arguments {
		return Arguments{}
	}
	_ component.Component = (*Component)(nil)

	// List of headers to ignore when copying headers from client to server connection
	// https://datatracker.ietf.org/doc/html/rfc9113#name-connection-specific-header-
	ignoreProxyHeaders = map[string]bool{
		"Connection":        true,
		"Proxy-Connection":  true,
		"Keep-Alive":        true,
		"Transfer-Encoding": true,
		"Upgrade":           true,
		"TE":                true,
	}
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.write",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			return New(o, c.(Arguments))
		},
	})
}

// Arguments represents the input state of the pyroscope.write
// component.
type Arguments struct {
	ExternalLabels map[string]string  `alloy:"external_labels,attr,optional"`
	Endpoints      []*EndpointOptions `alloy:"endpoint,block,optional"`
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
	opts    component.Options
	cfg     Arguments
	metrics *metrics
}

// Exports are the set of fields exposed by the pyroscope.write component.
type Exports struct {
	Receiver pyroscope.Appendable `alloy:"receiver,attr"`
}

// New creates a new pyroscope.write component.
func New(o component.Options, c Arguments) (*Component, error) {
	metrics := newMetrics(o.Registerer)
	receiver, err := NewFanOut(o, c, metrics)
	if err != nil {
		return nil, err
	}
	// Immediately export the receiver
	o.OnStateChange(Exports{Receiver: receiver})

	return &Component{
		cfg:     c,
		opts:    o,
		metrics: metrics,
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
	c.cfg = newConfig.(Arguments)
	level.Debug(c.opts.Logger).Log("msg", "updating pyroscope.write config", "old", c.cfg, "new", newConfig)
	receiver, err := NewFanOut(c.opts, newConfig.(Arguments), c.metrics)
	if err != nil {
		return err
	}
	c.opts.OnStateChange(Exports{Receiver: receiver})
	return nil
}

type fanOutClient struct {
	// The list of push clients to fan out to.
	pushClients   []pushv1connect.PusherServiceClient
	ingestClients map[*EndpointOptions]*http.Client
	config        Arguments
	opts          component.Options
	metrics       *metrics
}

// NewFanOut creates a new fan out client that will fan out to all endpoints.
func NewFanOut(opts component.Options, config Arguments, metrics *metrics) (*fanOutClient, error) {
	pushClients := make([]pushv1connect.PusherServiceClient, 0, len(config.Endpoints))
	ingestClients := make(map[*EndpointOptions]*http.Client)
	uid := alloyseed.Get().UID

	for _, endpoint := range config.Endpoints {
		if endpoint.Headers == nil {
			endpoint.Headers = map[string]string{}
		}
		endpoint.Headers[alloyseed.LegacyHeaderName] = uid
		endpoint.Headers[alloyseed.HeaderName] = uid
		httpClient, err := commonconfig.NewClientFromConfig(*endpoint.HTTPClientConfig.Convert(), endpoint.Name)
		if err != nil {
			return nil, err
		}
		pushClients = append(
			pushClients,
			pushv1connect.NewPusherServiceClient(httpClient, endpoint.URL, WithUserAgent(userAgent)),
		)
		ingestClients[endpoint] = httpClient
	}
	return &fanOutClient{
		pushClients:   pushClients,
		ingestClients: ingestClients,
		config:        config,
		opts:          opts,
		metrics:       metrics,
	}, nil
}

// Push implements the PusherServiceClient interface.
func (f *fanOutClient) Push(
	ctx context.Context,
	req *connect.Request[pushv1.PushRequest],
) (*connect.Response[pushv1.PushResponse], error) {
	// Don't flow the context down to the `run.Group`.
	// We want to fan out to all even in case of failures to one.
	var (
		g                     run.Group
		errs                  error
		reqSize, profileCount = requestSize(req)
	)

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
		g.Add(func() error {
			req := connect.NewRequest(req.Msg)
			for k, v := range f.config.Endpoints[i].Headers {
				req.Header().Set(k, v)
			}
			for {
				err = func() error {
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
				level.Warn(f.opts.Logger).
					Log("msg", "failed to push to endpoint", "endpoint", f.config.Endpoints[i].URL, "err", err)
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
				level.Warn(f.opts.Logger).
					Log("msg", "final error sending to profiles to endpoint", "endpoint", f.config.Endpoints[i].URL, "err", err)
				errs = multierr.Append(errs, err)
			}
			return err
		}, func(err error) {})
	}
	if err := g.Run(); err != nil {
		return nil, err
	}
	if errs != nil {
		return nil, errs
	}
	return connect.NewResponse(&pushv1.PushResponse{}), nil
}

func shouldRetry(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
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
		protoLabels  = make([]*typesv1.LabelPair, 0, len(lbs)+len(f.config.ExternalLabels))
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
	for name, value := range f.config.ExternalLabels {
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
	StatusCode int
}

func (e *PyroscopeWriteError) Error() string {
	return fmt.Sprintf("pyroscope write error: status %d", e.StatusCode)
}

// AppendIngest implements the pyroscope.Appender interface.
func (f *fanOutClient) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	g, ctx := errgroup.WithContext(ctx)

	// Send to each endpoint concurrently
	for _, endpoint := range f.config.Endpoints {
		g.Go(func() error {
			u, err := url.Parse(endpoint.URL)
			if err != nil {
				return fmt.Errorf("parse endpoint URL: %w", err)
			}

			u.Path = path.Join(u.Path, profile.URL.Path)

			// Handle labels
			query := profile.URL.Query()
			if !profile.Labels.IsEmpty() {
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
			}
			u.RawQuery = query.Encode()

			req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(profile.RawBody))
			if err != nil {
				return fmt.Errorf("create request: %w", err)
			}

			// First set profile headers as defaults
			for k, v := range profile.Headers {
				// Ignore this header as it may interfere with keepalives in the connection to pyroscope
				// which may cause huge load due to tls renegotiation
				if _, exists := ignoreProxyHeaders[k]; exists {
					continue
				}
				req.Header[k] = v
			}

			// Override any profile duplicated header
			for k, v := range endpoint.Headers {
				req.Header.Set(k, v)
			}

			resp, err := f.ingestClients[endpoint].Do(req)
			if err != nil {
				return fmt.Errorf("do request: %w", err)
			}
			defer resp.Body.Close()

			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				return fmt.Errorf("read response body: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				return &PyroscopeWriteError{StatusCode: resp.StatusCode}
			}
			return nil
		})
	}

	return g.Wait()
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
