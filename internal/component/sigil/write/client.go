package write

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/sigil-sdk/go/proto/sigil/wire"
	promconfig "github.com/prometheus/common/config"
)

const maxResponseBodyOverhead = 1024 * 1024 // 1 MiB

type endpointClient struct {
	options    *EndpointOptions
	httpClient *http.Client
	endpoint   string
	// label is the value used for the metric "endpoint" label: the configured
	// name, or the URL when name is unset.
	label   string
	logger  *slog.Logger
	metrics *metrics
}

func newEndpointClient(logger *slog.Logger, opts *EndpointOptions, m *metrics) (*endpointClient, error) {
	httpClient, err := promconfig.NewClientFromConfig(*opts.HTTPClientConfig.Convert(), opts.Name)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client for %s: %w", opts.URL, err)
	}

	endpoint, err := wire.NormalizeGenerationExportURL(opts.URL, opts.Insecure)
	if err != nil {
		return nil, fmt.Errorf("normalizing endpoint %s: %w", opts.URL, err)
	}

	label := opts.Name
	if label == "" {
		label = opts.URL
	}

	return &endpointClient{
		options:    opts,
		httpClient: httpClient,
		endpoint:   endpoint,
		label:      label,
		logger:     logger,
		metrics:    m,
	}, nil
}

func (ec *endpointClient) send(ctx context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	body, err := sigil.MarshalGenerationsRequest(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: ec.options.MinBackoff,
		MaxBackoff: ec.options.MaxBackoff,
		MaxRetries: ec.options.MaxBackoffRetries,
	})

	var lastErr error
	for {
		resp, err := ec.doRequest(ctx, req, body)
		if err == nil {
			ec.metrics.sentBytes.WithLabelValues(ec.label).Add(float64(len(body)))
			return resp, nil
		}

		lastErr = err
		ec.logger.Debug(
			"failed to send generations",
			"endpoint", ec.options.URL,
			"retries", bo.NumRetries(),
			"err", err,
		)

		if !shouldRetry(ctx, err) {
			break
		}
		bo.Wait()
		if !bo.Ongoing() {
			break
		}
		ec.metrics.retries.WithLabelValues(ec.label).Inc()
	}

	ec.metrics.droppedBytes.WithLabelValues(ec.label).Add(float64(len(body)))
	return nil, fmt.Errorf("failed to send to %s (%d retries): %w", ec.options.URL, bo.NumRetries(), lastErr)
}

func (ec *endpointClient) doRequest(ctx context.Context, req *sigil.GenerationsRequest, body []byte) (*sigil.GenerationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, ec.options.RemoteTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ec.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Apply endpoint-configured headers first so the Content-Type set below wins.
	for k, v := range ec.options.Headers {
		httpReq.Header.Set(k, v)
	}

	// The body is always protojson, so the endpoint can't override Content-Type.
	httpReq.Header.Set("Content-Type", wire.ContentTypeJSON)

	if ec.options.TenantID != "" {
		httpReq.Header.Set(wire.TenantHeaderName, ec.options.TenantID)
	} else if req.OrgID != "" && httpReq.Header.Get(wire.TenantHeaderName) == "" {
		httpReq.Header.Set(wire.TenantHeaderName, req.OrgID)
	}

	httpResp, err := ec.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		_ = httpResp.Body.Close()
	}()

	ec.metrics.requests.WithLabelValues(ec.label, strconv.Itoa(httpResp.StatusCode)).Inc()

	if httpResp.StatusCode/100 != 2 {
		errBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 2048))
		return nil, &WriteError{
			StatusCode: httpResp.StatusCode,
			Message:    string(errBody),
		}
	}

	// The response is one small ack per generation, so it should never exceed
	// the request that produced it by more than a small margin. Bound the read
	// to guard against an upstream returning an unbounded body.
	limit := int64(len(body)) + maxResponseBodyOverhead
	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if int64(len(respBody)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}

	parsed, err := sigil.ParseGenerationsResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &sigil.GenerationsResponse{
		StatusCode: httpResp.StatusCode,
		Response:   parsed,
	}, nil
}

// ExportGenerations sends the request to a single endpoint, recording
// per-endpoint latency. It makes endpointClient implement
// sigil.GenerationsForwarder so the shared FanOut helper can drive multi-
// endpoint fanout from this package.
func (ec *endpointClient) ExportGenerations(ctx context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	start := time.Now()
	resp, err := ec.send(ctx, req)
	ec.metrics.latency.WithLabelValues(ec.label).Observe(time.Since(start).Seconds())
	if err != nil {
		ec.logger.Warn("failed to send generations", "endpoint", ec.options.URL, "err", err)
	}
	return resp, err
}

// fanOutClient fans out generation requests to multiple endpoints.
type fanOutClient struct {
	endpoints []*endpointClient
	receivers []sigil.GenerationsForwarder
}

func (f *fanOutClient) closeIdleConnections() {
	for _, ec := range f.endpoints {
		ec.httpClient.CloseIdleConnections()
	}
}

func newFanOutClient(logger *slog.Logger, config Arguments, m *metrics) (*fanOutClient, error) {
	endpoints := make([]*endpointClient, 0, len(config.Endpoints))
	receivers := make([]sigil.GenerationsForwarder, 0, len(config.Endpoints))
	for _, ep := range config.Endpoints {
		ec, err := newEndpointClient(logger, ep, m)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ec)
		receivers = append(receivers, ec)
	}

	return &fanOutClient{
		endpoints: endpoints,
		receivers: receivers,
	}, nil
}

func (f *fanOutClient) ExportGenerations(ctx context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	return sigil.FanOut(ctx, req, f.receivers)
}

func shouldRetry(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}

	var writeErr *WriteError
	if errors.As(err, &writeErr) {
		return isRetryableStatus(writeErr.StatusCode)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusRequestTimeout ||
		status >= http.StatusInternalServerError
}

// WriteError represents an HTTP error from the upstream Sigil endpoint.
type WriteError struct {
	StatusCode int
	Message    string
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("sigil write error: status=%d msg=%s", e.StatusCode, e.Message)
}
