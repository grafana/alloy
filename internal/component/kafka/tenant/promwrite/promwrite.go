// Package promwrite implements kafka.tenant_prometheus_write, a minimal
// tenant-aware Prometheus remote-write sink. Each appender transaction is sent
// synchronously on Commit as one remote-write v1 request, with X-Scope-OrgID
// taken from the context the appender was created with.
package promwrite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/client_golang/prometheus"
	promconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/alloy/internal/util"
)

func init() {
	component.Register(component.Registration{
		Name:      "kafka.tenant_prometheus_write",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures kafka.tenant_prometheus_write.
type Arguments struct {
	Endpoint EndpointOptions `alloy:"endpoint,block"`
}

// EndpointOptions configures the remote-write endpoint.
type EndpointOptions struct {
	URL              string                   `alloy:"url,attr"`
	RemoteTimeout    time.Duration            `alloy:"remote_timeout,attr,optional"`
	Headers          map[string]string        `alloy:"headers,attr,optional"`
	MinBackoff       time.Duration            `alloy:"min_backoff_period,attr,optional"`
	MaxBackoff       time.Duration            `alloy:"max_backoff_period,attr,optional"`
	MaxRetries       int                      `alloy:"max_backoff_retries,attr,optional"`
	HTTPClientConfig *config.HTTPClientConfig `alloy:",squash"`
}

// SetToDefault implements syntax.Defaulter.
func (e *EndpointOptions) SetToDefault() {
	*e = EndpointOptions{
		RemoteTimeout:    30 * time.Second,
		MinBackoff:       100 * time.Millisecond,
		MaxBackoff:       5 * time.Second,
		MaxRetries:       5,
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}
}

// Validate implements syntax.Validator.
func (e *EndpointOptions) Validate() error {
	if e.URL == "" {
		return errors.New("url must be set")
	}
	// HTTPClientConfig is squashed, so it must be validated explicitly.
	if e.HTTPClientConfig != nil {
		return e.HTTPClientConfig.Validate()
	}
	return nil
}

// Exports are the values exported by kafka.tenant_prometheus_write.
type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// Component implements kafka.tenant_prometheus_write.
type Component struct {
	opts    component.Options
	metrics *metrics

	mut      sync.RWMutex
	endpoint EndpointOptions
	client   *http.Client
}

var _ storage.Appendable = (*Component)(nil)

// New creates a new kafka.tenant_prometheus_write component.
func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{opts: opts, metrics: newMetrics(opts.Registerer)}
	if err := c.Update(args); err != nil {
		return nil, err
	}
	opts.OnStateChange(Exports{Receiver: c})
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	client, err := promconfig.NewClientFromConfig(*newArgs.Endpoint.HTTPClientConfig.Convert(), "kafka_tenant_prometheus_write")
	if err != nil {
		return err
	}

	c.mut.Lock()
	defer c.mut.Unlock()
	c.endpoint = newArgs.Endpoint
	c.client = client
	return nil
}

// Appender implements storage.Appendable. The tenant is read from ctx on
// Commit.
func (c *Component) Appender(ctx context.Context) storage.Appender {
	return &appender{ctx: ctx, c: c, idx: map[uint64]int{}}
}

// send POSTs one remote-write request for tenant, retrying 5xx, 429 and
// network errors with bounded backoff.
func (c *Component) send(ctx context.Context, tenant string, body []byte, samples int) error {
	c.mut.RLock()
	endpoint, client := c.endpoint, c.client
	c.mut.RUnlock()

	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: endpoint.MinBackoff,
		MaxBackoff: endpoint.MaxBackoff,
		MaxRetries: endpoint.MaxRetries,
	})

	var lastErr error
	for bo.Ongoing() {
		code, err := c.sendOnce(ctx, client, endpoint, tenant, body)
		c.metrics.requests.WithLabelValues(strconv.Itoa(code)).Inc()
		if err == nil {
			c.metrics.sentSamples.Add(float64(samples))
			return nil
		}
		lastErr = err
		if code != 0 && code != http.StatusTooManyRequests && code < 500 {
			return err
		}
		bo.Wait()
	}
	if lastErr == nil {
		lastErr = bo.Err()
	}
	return fmt.Errorf("remote write failed after %d retries: %w", bo.NumRetries(), lastErr)
}

func (c *Component) sendOnce(ctx context.Context, client *http.Client, endpoint EndpointOptions, tenant string, body []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, endpoint.RemoteTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	for k, v := range endpoint.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("User-Agent", useragent.Get())
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	req.Header.Set(user.OrgIDHeaderName, tenant)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode/100 != 2 {
		return resp.StatusCode, fmt.Errorf("server returned HTTP status %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	return resp.StatusCode, nil
}

// appender buffers one transaction and sends it on Commit.
type appender struct {
	ctx     context.Context
	c       *Component
	req     prompb.WriteRequest
	idx     map[uint64]int
	samples int
}

var _ storage.Appender = (*appender)(nil)

func (a *appender) series(l labels.Labels) *prompb.TimeSeries {
	h := l.Hash()
	i, ok := a.idx[h]
	if !ok {
		a.req.Timeseries = append(a.req.Timeseries, prompb.TimeSeries{Labels: prompb.FromLabels(l, nil)})
		i = len(a.req.Timeseries) - 1
		a.idx[h] = i
	}
	return &a.req.Timeseries[i]
}

func ref(ref storage.SeriesRef, l labels.Labels) storage.SeriesRef {
	if ref != 0 {
		return ref
	}
	return storage.SeriesRef(l.Hash())
}

func (a *appender) Append(r storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	ts := a.series(l)
	ts.Samples = append(ts.Samples, prompb.Sample{Timestamp: t, Value: v})
	a.samples++
	return ref(r, l), nil
}

func (a *appender) AppendExemplar(r storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	ts := a.series(l)
	ts.Exemplars = append(ts.Exemplars, prompb.Exemplar{
		Labels:    prompb.FromLabels(e.Labels, nil),
		Value:     e.Value,
		Timestamp: e.Ts,
	})
	return ref(r, l), nil
}

func (a *appender) AppendHistogram(r storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	ts := a.series(l)
	if h != nil {
		ts.Histograms = append(ts.Histograms, prompb.FromIntHistogram(t, h))
	} else if fh != nil {
		ts.Histograms = append(ts.Histograms, prompb.FromFloatHistogram(t, fh))
	}
	a.samples++
	return ref(r, l), nil
}

func (a *appender) AppendHistogramSTZeroSample(r storage.SeriesRef, l labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref(r, l), nil
}

func (a *appender) AppendSTZeroSample(r storage.SeriesRef, l labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return ref(r, l), nil
}

func (a *appender) UpdateMetadata(r storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	a.req.Metadata = append(a.req.Metadata, prompb.MetricMetadata{
		Type:             prompb.FromMetadataType(m.Type),
		MetricFamilyName: l.Get(model.MetricNameLabel),
		Help:             m.Help,
		Unit:             m.Unit,
	})
	return ref(r, l), nil
}

func (a *appender) SetOptions(*storage.AppendOptions) {}

// Commit sends the transaction synchronously, so a caller that commits Kafka
// offsets after Commit returns gets at-least-once delivery.
func (a *appender) Commit() error {
	defer a.reset()
	if len(a.req.Timeseries) == 0 && len(a.req.Metadata) == 0 {
		return nil
	}

	tenant, err := user.ExtractOrgID(a.ctx)
	if err != nil {
		return fmt.Errorf("no tenant in appender context: %w", err)
	}

	raw, err := a.req.Marshal()
	if err != nil {
		return err
	}
	return a.c.send(a.ctx, tenant, snappy.Encode(nil, raw), a.samples)
}

func (a *appender) Rollback() error {
	a.reset()
	return nil
}

func (a *appender) reset() {
	a.req = prompb.WriteRequest{}
	a.idx = map[uint64]int{}
	a.samples = 0
}

type metrics struct {
	requests    *prometheus.CounterVec
	sentSamples prometheus.Counter
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kafka_tenant_prometheus_write_requests_total",
			Help: "Total number of remote-write requests by response code (0 for network errors).",
		}, []string{"code"}),
		sentSamples: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "kafka_tenant_prometheus_write_sent_samples_total",
			Help: "Total number of samples and histograms successfully sent.",
		}),
	}
	m.requests = util.MustRegisterOrGet(reg, m.requests).(*prometheus.CounterVec)
	m.sentSamples = util.MustRegisterOrGet(reg, m.sentSamples).(prometheus.Counter)
	return m
}
