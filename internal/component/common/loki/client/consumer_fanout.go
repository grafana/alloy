package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/useragent"
	"github.com/grafana/dskit/backoff"
)

func NewFanoutConsumer(logger log.Logger, reg prometheus.Registerer, clientCfgs ...Config) (*FanoutConsumer, error) {
	if len(clientCfgs) == 0 {
		return nil, fmt.Errorf("at least one client config must be provided")
	}

	m := &FanoutConsumer{
		clients: make([]*client, 0, len(clientCfgs)),
		recv:    make(chan loki.Entry),
	}

	var (
		metrics      = NewMetrics(reg)
		clientsCheck = make(map[string]struct{})
	)

	for _, cfg := range clientCfgs {
		// Don't allow duplicate clients, we have client specific metrics that need at least one unique label value (name).
		clientName := getClientName(cfg)
		if _, ok := clientsCheck[clientName]; ok {
			return nil, fmt.Errorf("duplicate client configs are not allowed, found duplicate for name: %s", cfg.Name)
		}

		clientsCheck[clientName] = struct{}{}
		client, err := newClient(metrics, cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("error starting client: %w", err)
		}

		m.clients = append(m.clients, client)
	}

	m.wg.Go(m.run)
	return m, nil
}

var _ Consumer = (*FanoutConsumer)(nil)

type FanoutConsumer struct {
	clients []*client
	wg      sync.WaitGroup
	once    sync.Once
	recv    chan loki.Entry
}

func (c *FanoutConsumer) run() {
	for e := range c.recv {
		for _, c := range c.clients {
			c.Chan() <- e
		}
	}
}

func (c *FanoutConsumer) Chan() chan<- loki.Entry {
	return c.recv
}

func (c *FanoutConsumer) Stop() {
	// First stop the receiving channel.
	c.once.Do(func() { close(c.recv) })
	c.wg.Wait()

	var stopWG sync.WaitGroup
	// Stop all clients.
	for _, c := range c.clients {
		stopWG.Go(func() {
			c.Stop()
		})
	}

	// Wait for all clients to stop.
	stopWG.Wait()
}

// getClientName computes the specific name for each client config. The name is either the configured Name setting in Config,
// or a hash of the config as whole, this allows us to detect repeated configs.
func getClientName(cfg Config) string {
	if cfg.Name != "" {
		return cfg.Name
	}
	return asSha256(cfg)
}

func asSha256(o interface{}) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%v", o)

	temp := fmt.Sprintf("%x", h.Sum(nil))
	return temp[:6]
}

const (
	contentType  = "application/x-protobuf"
	maxErrMsgLen = 1024

	// Label reserved to override the tenant ID while processing
	// pipeline stages
	ReservedLabelTenantID = "__tenant_id__"
)

var userAgent = useragent.Get()

// Client for pushing logs in snappy-compressed protos over HTTP.
type client struct {
	metrics *Metrics
	logger  log.Logger
	cfg     Config
	client  *http.Client
	entries chan loki.Entry

	once sync.Once
	wg   sync.WaitGroup

	// ctx is used in any upstream calls from the `client`.
	ctx    context.Context
	cancel context.CancelFunc
}

func newClient(metrics *Metrics, cfg Config, logger log.Logger) (*client, error) {
	if cfg.URL.URL == nil {
		return nil, errors.New("client needs target URL")
	}
	if metrics == nil {
		return nil, errors.New("metrics must be instantiated")
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &client{
		logger:  log.With(logger, "component", "client", "host", cfg.URL.Host),
		cfg:     cfg,
		entries: make(chan loki.Entry),
		metrics: metrics,
		ctx:     ctx,
		cancel:  cancel,
	}

	err := cfg.Client.Validate()
	if err != nil {
		return nil, err
	}

	c.client, err = config.NewClientFromConfig(cfg.Client, useragent.ProductName, config.WithHTTP2Disabled())
	if err != nil {
		return nil, err
	}

	c.client.Timeout = cfg.Timeout

	c.wg.Go(func() { c.run() })
	return c, nil
}

func (c *client) initBatchMetrics(tenantID string) {
	// Initialize counters to 0 so the metrics are exported before the first
	// occurrence of incrementing to avoid missing metrics.
	for _, counter := range c.metrics.countersWithHostTenantReason {
		for _, reason := range Reasons {
			counter.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Add(0)
		}
	}

	for _, counter := range c.metrics.countersWithHostTenant {
		counter.WithLabelValues(c.cfg.URL.Host, tenantID).Add(0)
	}
}

func (c *client) run() {
	batches := map[string]*batch{}

	// Given the client handles multiple batches (1 per tenant) and each batch
	// can be created at a different point in time, we look for batches whose
	// max wait time has been reached every 10 times per BatchWait, so that the
	// maximum delay we have sending batches is 10% of the max waiting time.
	// We apply a cap of 10ms to the ticker, to avoid too frequent checks in
	// case the BatchWait is very low.
	minWaitCheckFrequency := 10 * time.Millisecond
	maxWaitCheckFrequency := max(c.cfg.BatchWait/10, minWaitCheckFrequency)

	maxWaitCheck := time.NewTicker(maxWaitCheckFrequency)

	defer func() {
		maxWaitCheck.Stop()
		// Send all pending batches
		for tenantID, batch := range batches {
			c.sendBatch(tenantID, batch)
		}
	}()

	for {
		select {
		case e, ok := <-c.entries:
			if !ok {
				return
			}

			e, tenantID := c.processEntry(e)
			batch, ok := batches[tenantID]

			// If the batch doesn't exist yet, we create a new one with the entry
			if !ok {
				batches[tenantID] = newBatch(c.cfg.MaxStreams, e)
				c.initBatchMetrics(tenantID)
				break
			}

			// If adding the entry to the batch will increase the size over the max
			// size allowed, we do send the current batch and then create a new one
			if batch.sizeBytesAfter(e.Entry) > c.cfg.BatchSize {
				c.sendBatch(tenantID, batch)

				batches[tenantID] = newBatch(c.cfg.MaxStreams, e)
				break
			}

			// The max size of the batch isn't reached, so we can add the entry
			err := batch.add(e, 0)
			if err != nil {
				level.Error(c.logger).Log("msg", "batch add err", "tenant", tenantID, "error", err)
				reason := ReasonGeneric
				if errors.Is(err, errMaxStreamsLimitExceeded) {
					reason = ReasonStreamLimited
				}
				c.metrics.droppedBytes.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Add(float64(len(e.Line)))
				c.metrics.droppedEntries.WithLabelValues(c.cfg.URL.Host, tenantID, reason).Inc()
				return
			}
		case <-maxWaitCheck.C:
			// Send all batches whose max wait time has been reached
			for tenantID, batch := range batches {
				if batch.age() < c.cfg.BatchWait {
					continue
				}

				c.sendBatch(tenantID, batch)
				delete(batches, tenantID)
			}
		}
	}
}

func (c *client) Chan() chan<- loki.Entry {
	return c.entries
}

func batchIsRateLimited(status int) bool {
	return status == 429
}

func (c *client) sendBatch(tenantID string, batch *batch) {
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
		status, err = c.send(context.Background(), tenantID, buf)

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

func (c *client) send(ctx context.Context, tenantID string, buf []byte) (int, error) {
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

	// NOTE: it is important in goland to fully ready the body and
	// close it so that the connection can be reused.
	// We only partailly read the body if we encounter a non 2xx error
	// so we should always consume whats left.
	// https://github.com/golang/go/blob/32a9804c7ba3f4a0e0bd26cc24b9204860a49ec8/src/net/http/response.go#L59-L64
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

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

func (c *client) getTenantID(labels model.LabelSet) string {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	// Check if has been specified in the config
	if c.cfg.TenantID != "" {
		return c.cfg.TenantID
	}

	// Defaults to an empty string, which means the X-Scope-OrgID header
	// will not be sent
	return ""
}

// Stop the client.
func (c *client) Stop() {
	c.once.Do(func() { close(c.entries) })
	c.wg.Wait()
}

func (c *client) processEntry(e loki.Entry) (loki.Entry, string) {
	tenantID := c.getTenantID(e.Labels)
	return e, tenantID
}
