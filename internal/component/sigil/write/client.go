package write

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

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/sigil"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/dskit/backoff"
	promconfig "github.com/prometheus/common/config"
)

type endpointClient struct {
	options    *EndpointOptions
	httpClient *http.Client
	logger     log.Logger
	metrics    *metrics
}

func newEndpointClient(logger log.Logger, opts *EndpointOptions, m *metrics) (*endpointClient, error) {
	httpClient, err := promconfig.NewClientFromConfig(*opts.HTTPClientConfig.Convert(), opts.Name)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client for %s: %w", opts.URL, err)
	}

	return &endpointClient{
		options:    opts,
		httpClient: httpClient,
		logger:     logger,
		metrics:    m,
	}, nil
}

func (ec *endpointClient) send(ctx context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	bo := backoff.New(ctx, backoff.Config{
		MinBackoff: ec.options.MinBackoff,
		MaxBackoff: ec.options.MaxBackoff,
		MaxRetries: ec.options.MaxBackoffRetries,
	})

	var lastErr error
	for {
		resp, err := ec.doRequest(ctx, req)
		if err == nil {
			ec.metrics.sentBytes.WithLabelValues(ec.options.URL).Add(float64(len(req.Body)))
			ec.metrics.requests.WithLabelValues(ec.options.URL, strconv.Itoa(resp.StatusCode)).Inc()
			return resp, nil
		}

		lastErr = err
		level.Debug(ec.logger).Log(
			"msg", "failed to send generations",
			"endpoint", ec.options.URL,
			"retries", bo.NumRetries(),
			"err", err,
		)

		if !shouldRetry(err) {
			break
		}
		bo.Wait()
		if !bo.Ongoing() {
			break
		}
		ec.metrics.retries.WithLabelValues(ec.options.URL).Inc()
	}

	ec.metrics.droppedBytes.WithLabelValues(ec.options.URL).Add(float64(len(req.Body)))
	return nil, fmt.Errorf("failed to send to %s (%d retries): %w", ec.options.URL, bo.NumRetries(), lastErr)
}

func (ec *endpointClient) doRequest(ctx context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, ec.options.RemoteTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ec.options.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Apply endpoint-configured headers first so request content-type wins.
	for k, v := range ec.options.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set content type from the incoming request, overwriting any endpoint header.
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}

	// Set tenant ID: config overrides request metadata.
	if ec.options.TenantID != "" {
		httpReq.Header.Set("X-Scope-OrgID", ec.options.TenantID)
	} else if req.OrgID != "" {
		httpReq.Header.Set("X-Scope-OrgID", req.OrgID)
	}

	// Forward additional headers from the original request.
	for k, v := range req.Headers {
		if httpReq.Header.Get(k) == "" {
			httpReq.Header.Set(k, v)
		}
	}

	httpResp, err := ec.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 2048))
		return nil, &WriteError{
			StatusCode: httpResp.StatusCode,
			Message:    string(errBody),
		}
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &sigil.GenerationsResponse{
		StatusCode: httpResp.StatusCode,
		Body:       respBody,
	}, nil
}

// fanOutClient fans out generation requests to multiple endpoints.
type fanOutClient struct {
	endpoints []*endpointClient
	metrics   *metrics
	logger    log.Logger
}

func (f *fanOutClient) closeIdleConnections() {
	for _, ec := range f.endpoints {
		ec.httpClient.CloseIdleConnections()
	}
}

func newFanOutClient(logger log.Logger, config Arguments, m *metrics) (*fanOutClient, error) {
	endpoints := make([]*endpointClient, 0, len(config.Endpoints))
	for _, ep := range config.Endpoints {
		ec, err := newEndpointClient(logger, ep, m)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ec)
	}

	return &fanOutClient{
		endpoints: endpoints,
		metrics:   m,
		logger:    logger,
	}, nil
}

func (f *fanOutClient) ExportGenerations(ctx context.Context, req *sigil.GenerationsRequest) (*sigil.GenerationsResponse, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    error
		resp    *sigil.GenerationsResponse
		reqSize = int64(len(req.Body))
	)

	for _, ec := range f.endpoints {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			r, err := ec.send(ctx, req)
			ec.metrics.latency.WithLabelValues(ec.options.URL).Observe(time.Since(start).Seconds())

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				level.Warn(ec.logger).Log("msg", "failed to send generations", "endpoint", ec.options.URL, "sz", reqSize, "err", err)
				errs = errors.Join(errs, err)
			} else if resp == nil {
				resp = r
			}
		}()
	}

	wg.Wait()
	if resp == nil && errs != nil {
		return nil, errs
	}
	return resp, errs
}

func shouldRetry(err error) bool {
	var writeErr *WriteError
	if errors.As(err, &writeErr) {
		s := writeErr.StatusCode
		return s == http.StatusTooManyRequests ||
			s == http.StatusRequestTimeout ||
			s >= http.StatusInternalServerError
	}
	return true // network errors are retryable
}

// WriteError represents an HTTP error from the upstream Sigil endpoint.
type WriteError struct {
	StatusCode int
	Message    string
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("sigil write error: status=%d msg=%s", e.StatusCode, e.Message)
}
