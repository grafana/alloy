package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	lokiutil "github.com/grafana/loki/v3/pkg/util"
	"github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
)

const (
	contentType  = "application/x-protobuf"
	maxErrMsgLen = 1024
)

func newBatchClient(m *Metrics, logger log.Logger, cfg Config) (*batchClient, error) {
	if cfg.URL.URL == nil {
		return nil, errors.New("client needs target URL")
	}

	err := cfg.Client.Validate()
	if err != nil {
		return nil, err
	}

	httpClient, err := config.NewClientFromConfig(cfg.Client, useragent.ProductName, config.WithHTTP2Disabled())
	if err != nil {
		return nil, err
	}

	httpClient.Timeout = cfg.Timeout

	ctx, cancel := context.WithCancel(context.Background())
	return &batchClient{
		cfg:     cfg,
		metrics: m,
		logger:  logger,
		client:  httpClient,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

type batchClient struct {
	cfg     Config
	metrics *Metrics
	logger  log.Logger

	client *http.Client

	ctx    context.Context
	cancel context.CancelFunc
}

func (c *batchClient) sendBatch(ctx context.Context, tenantID string, batch *batch) {
	buf, entriesCount, err := batch.encode()
	if err != nil {
		level.Error(c.logger).Log("msg", "error encoding batch", "error", err)
		return
	}
	bufBytes := float64(len(buf))
	c.metrics.encodedBytes.WithLabelValues(c.cfg.URL.Host, tenantID).Add(bufBytes)

	backoff := backoff.New(c.ctx, c.cfg.BackoffConfig)
	var status int
	for {
		start := time.Now()
		// send uses `timeout` internally, so `context.Background` is good enough.
		status, err = c.send(ctx, tenantID, buf)

		c.metrics.requestDuration.WithLabelValues(strconv.Itoa(status), c.cfg.URL.Host, tenantID).Observe(time.Since(start).Seconds())

		// Immediately drop rate limited batches to avoid HOL blocking for other tenants not experiencing throttling
		if c.cfg.DropRateLimitedBatches && batchIsRateLimited(status) {
			level.Warn(c.logger).Log("msg", "dropping batch due to rate limiting applied at ingester")
			c.metrics.droppedBytes.WithLabelValues(c.cfg.URL.Host, tenantID, ReasonRateLimited).Add(bufBytes)
			c.metrics.droppedEntries.WithLabelValues(c.cfg.URL.Host, tenantID, ReasonRateLimited).Add(float64(entriesCount))
			return
		}

		if err == nil {
			c.metrics.sentBytes.WithLabelValues(c.cfg.URL.Host, tenantID).Add(bufBytes)
			c.metrics.sentEntries.WithLabelValues(c.cfg.URL.Host, tenantID).Add(float64(entriesCount))

			return
		}

		// Only retry 429s, 500s and connection-level errors.
		if status > 0 && !batchIsRateLimited(status) && status/100 != 5 {
			break
		}

		level.Debug(c.logger).Log("msg", "error sending batch, will retry", "status", status, "tenant", tenantID, "error", err)
		c.metrics.batchRetries.WithLabelValues(c.cfg.URL.Host, tenantID).Inc()
		backoff.Wait()

		// Make sure it sends at least once before checking for retry.
		if !backoff.Ongoing() {
			break
		}
	}

	level.Error(c.logger).Log("msg", "final error sending batch, no retries left, dropping data", "status", status, "tenant", tenantID, "error", err)
	// If the reason for the last retry error was rate limiting, count the drops as such, even if the previous errors
	// were for a different reason
	dropReason := ReasonGeneric
	if batchIsRateLimited(status) {
		dropReason = ReasonRateLimited
	}
	c.metrics.droppedBytes.WithLabelValues(c.cfg.URL.Host, tenantID, dropReason).Add(bufBytes)
	c.metrics.droppedEntries.WithLabelValues(c.cfg.URL.Host, tenantID, dropReason).Add(float64(entriesCount))
}

func (c *batchClient) send(ctx context.Context, tenantID string, buf []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.URL.String(), bytes.NewReader(buf))
	if err != nil {
		return -1, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", userAgent)

	// If the tenant ID is not empty promtail is running in multi-tenant mode, so
	// we should send it to Loki
	if tenantID != "" {
		req.Header.Set("X-Scope-OrgID", tenantID)
	}

	// Add custom headers on request
	if len(c.cfg.Headers) > 0 {
		for k, v := range c.cfg.Headers {
			if req.Header.Get(k) == "" {
				req.Header.Add(k, v)
			} else {
				level.Warn(c.logger).Log("msg", "custom header key already exists, skipping", "key", k)
			}
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return -1, err
	}
	defer lokiutil.LogError("closing response body", resp.Body.Close)

	if resp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	}
	return resp.StatusCode, err
}

func (b *batchClient) stop() {
	b.cancel()
}

func batchIsRateLimited(status int) bool {
	return status == 429
}
