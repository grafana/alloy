// Package tailscale_exporter provides a Prometheus exporter for Tailscale
// tailnets. It joins the tailnet as an embedded node via tsnet, queries the
// Tailscale management API for device status, and scrapes per-node Tailscale
// daemon metrics from each peer.
package tailscale_exporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	tsclient "tailscale.com/client/tailscale/v2"
	"tailscale.com/tsnet"

	"github.com/grafana/alloy/internal/static/integrations/config"
)

const (
	defaultAPIBaseURL        = "https://api.tailscale.com"
	defaultRefreshInterval   = 60 * time.Second
	defaultPeerMetricsPort   = 5252
	defaultPeerMetricsPath   = "/metrics"
	defaultPeerScrapeTimeout = 3 * time.Second
	defaultTSNetHostname     = "alloy-tailscale-exporter"

	// onlineThreshold is the duration within which a device must have been
	// seen to be considered online.
	onlineThreshold = 5 * time.Minute
)

// Config holds the runtime configuration for the tailscale integration.
type Config struct {
	Tailnet           string
	APIKey            string
	AuthKey           string
	APIBaseURL        string
	StateDir          string
	TSNetHostname     string
	RefreshInterval   time.Duration
	PeerMetricsPort   int
	PeerMetricsPath   string
	PeerScrapeTimeout time.Duration

	// OAuthClientID / OAuthClientSecret, when set, replace APIKey and AuthKey.
	// The client credentials authenticate the management API (token minted via
	// the client-credentials flow) and the tsnet node join (tsnet mints tagged
	// auth keys via OAuth). AdvertiseTags are applied to the node and are
	// required with OAuth, since OAuth-generated auth keys must be tagged.
	OAuthClientID     string
	OAuthClientSecret string
	AdvertiseTags     []string

	// Targets maps node types (matched by tag glob) to the port/path where they
	// expose metrics — e.g. clients and exit nodes on :5252, k8s operator
	// proxies and ingresses on :9002. When empty, every device is scraped on
	// PeerMetricsPort/PeerMetricsPath.
	Targets []ScrapeTarget
}

// ScrapeTarget describes where a group of nodes exposes Prometheus metrics.
type ScrapeTarget struct {
	// MatchTags is a list of tag glob patterns (e.g. "tag:*-proxy"). A device
	// matches if any of its tags matches any pattern. An empty list matches
	// every device, so it acts as a catch-all.
	MatchTags []string
	Port      int
	Path      string
	// Labels are extra labels attached to every metric from matched nodes.
	Labels map[string]string
}

// integration implements integrations.Integration.
type integration struct {
	logger *slog.Logger
	cfg    Config

	// registry is swapped atomically on each refresh cycle. A nil value means
	// no successful refresh has occurred yet.
	registry atomicRegistry

	// peerMu guards peerCache.
	peerMu    sync.RWMutex
	peerCache map[string]peerEntry // peer hostname -> scraped metrics + labels

	// Self-reported health counters, registered once at construction and
	// included in every registry swap.
	peerScrapeErrors *prometheus.CounterVec
	apiErrors        prometheus.Counter
	lastRefreshTime  prometheus.Gauge
	lastRefreshDur   prometheus.Gauge
}

// atomicRegistry wraps atomic pointer operations for *prometheus.Registry.
type atomicRegistry struct {
	mu  sync.RWMutex
	reg *prometheus.Registry
}

func (a *atomicRegistry) store(r *prometheus.Registry) {
	a.mu.Lock()
	a.reg = r
	a.mu.Unlock()
}

func (a *atomicRegistry) load() *prometheus.Registry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.reg
}

// New creates a new tailscale integration.
func New(logger *slog.Logger, cfg Config) (*integration, error) {
	if cfg.Tailnet == "" {
		return nil, fmt.Errorf("tailnet is required")
	}
	if cfg.OAuthClientID != "" {
		if cfg.OAuthClientSecret == "" {
			return nil, fmt.Errorf("oauth client_secret is required")
		}
	} else {
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("api_key or oauth is required")
		}
		if cfg.AuthKey == "" {
			return nil, fmt.Errorf("auth_key is required")
		}
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = defaultRefreshInterval
	}
	if cfg.PeerMetricsPort == 0 {
		cfg.PeerMetricsPort = defaultPeerMetricsPort
	}
	if cfg.PeerMetricsPath == "" {
		cfg.PeerMetricsPath = defaultPeerMetricsPath
	}
	if cfg.PeerScrapeTimeout <= 0 {
		cfg.PeerScrapeTimeout = defaultPeerScrapeTimeout
	}
	if cfg.TSNetHostname == "" {
		cfg.TSNetHostname = defaultTSNetHostname
	}

	i := &integration{
		logger:    logger,
		cfg:       cfg,
		peerCache: make(map[string]peerEntry),

		peerScrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tailscale_exporter_peer_scrape_errors_total",
			Help: "Total number of errors scraping per-node Tailscale daemon metrics.",
		}, []string{"node"}),
		apiErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tailscale_exporter_api_errors_total",
			Help: "Total number of Tailscale management API call errors.",
		}),
		lastRefreshTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tailscale_exporter_last_refresh_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful refresh cycle.",
		}),
		lastRefreshDur: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tailscale_exporter_last_refresh_duration_seconds",
			Help: "Duration in seconds of the last full refresh cycle.",
		}),
	}

	return i, nil
}

// MetricsHandler implements integrations.Integration.
func (i *integration) MetricsHandler() (http.Handler, error) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reg := i.registry.load()
		if reg == nil {
			http.Error(w, "metrics not yet available", http.StatusServiceUnavailable)
			return
		}

		i.peerMu.RLock()
		peerSnap := copyPeerCache(i.peerCache)
		i.peerMu.RUnlock()

		merged := prometheus.Gatherers{reg, &peerMetricsGatherer{cache: peerSnap}}
		promhttp.HandlerFor(merged, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		}).ServeHTTP(w, r)
	})
	return h, nil
}

// ScrapeConfigs implements integrations.Integration.
func (i *integration) ScrapeConfigs() []config.ScrapeConfig {
	return []config.ScrapeConfig{{
		JobName:     "tailscale",
		MetricsPath: "/metrics",
	}}
}

// Run implements integrations.Integration. It starts the tsnet embedded node,
// then periodically refreshes API data and peer metrics until ctx is canceled.
func (i *integration) Run(ctx context.Context) error {
	if err := os.MkdirAll(i.cfg.StateDir, 0700); err != nil {
		return fmt.Errorf("create state dir %q: %w", i.cfg.StateDir, err)
	}

	srv := &tsnet.Server{
		Dir:      i.cfg.StateDir,
		Hostname: i.cfg.TSNetHostname,
		Logf: func(format string, args ...any) {
			i.logger.Debug(fmt.Sprintf(format, args...))
		},
	}
	if i.cfg.OAuthClientID != "" {
		// tsnet mints tagged auth keys via OAuth; no static auth key needed.
		srv.ClientID = i.cfg.OAuthClientID
		srv.ClientSecret = i.cfg.OAuthClientSecret
		srv.AdvertiseTags = i.cfg.AdvertiseTags
	} else {
		srv.AuthKey = i.cfg.AuthKey
	}
	if err := srv.Start(); err != nil {
		return fmt.Errorf("tsnet start: %w", err)
	}
	defer srv.Close()

	// tsHTTPClient routes all traffic through the tsnet VPN.
	tsHTTPClient := srv.HTTPClient()
	tsHTTPClient.Timeout = i.cfg.PeerScrapeTimeout

	apiClient, err := i.newAPIClient()
	if err != nil {
		return fmt.Errorf("build API client: %w", err)
	}

	// Run an initial refresh before the first tick so metrics are available
	// immediately after startup.
	i.refresh(ctx, apiClient, tsHTTPClient)

	ticker := time.NewTicker(i.cfg.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			i.refresh(ctx, apiClient, tsHTTPClient)
		}
	}
}

// refresh fetches device data from the API and scrapes peer metrics, then
// atomically updates the cached registry and peer text.
func (i *integration) refresh(ctx context.Context, apiClient *tsclient.Client, tsHTTPClient *http.Client) {
	start := time.Now()

	devices, err := apiClient.Devices().List(ctx)
	if err != nil {
		i.logger.Error("tailscale API call failed", "err", err)
		i.apiErrors.Inc()
		return
	}

	// Scrape per-node daemon metrics in parallel.
	newPeerCache := make(map[string]peerEntry, len(devices))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, d := range devices {
		if len(d.Addresses) == 0 {
			continue
		}
		port, path, extraLabels, ok := i.resolveTarget(d.Tags)
		if !ok {
			// No configured target group matches this node's tags — skip it.
			continue
		}
		d := d // capture loop var
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Addresses are in CIDR notation (e.g. "100.64.1.1/32"); strip the prefix.
			ip, _, _ := strings.Cut(d.Addresses[0], "/")
			url := fmt.Sprintf("http://%s:%d%s", ip, port, path)
			raw, scrapeErr := scrapePeer(ctx, tsHTTPClient, url)
			if scrapeErr != nil {
				var errno syscall.Errno
				connRefused := (errors.As(scrapeErr, &errno) && errno == syscall.ECONNREFUSED) ||
					strings.Contains(scrapeErr.Error(), "connection refused")
				if connRefused {
					// Connection refused means the peer doesn't expose metrics — this is
					// expected for nodes that haven't enabled the Tailscale metrics endpoint.
					i.logger.Debug("peer metrics not available", "node", d.Hostname, "err", scrapeErr)
				} else {
					i.logger.Warn("peer metrics scrape failed", "node", d.Hostname, "err", scrapeErr)
					i.peerScrapeErrors.WithLabelValues(d.Hostname).Inc()
				}
				return
			}
			labels := map[string]string{
				"tags": strings.Join(d.Tags, ","),
				"os":   d.OS,
			}
			for k, v := range extraLabels {
				labels[k] = v // per-target labels win over defaults
			}
			mu.Lock()
			newPeerCache[d.Hostname] = peerEntry{raw: raw, labels: labels}
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Build a fresh registry for API-level metrics.
	reg := prometheus.NewRegistry()
	if err := i.registerAPIMetrics(reg, devices); err != nil {
		i.logger.Error("failed to register API metrics", "err", err)
		i.apiErrors.Inc()
		return
	}

	// Register health metrics into the same registry.
	dur := time.Since(start).Seconds()
	i.lastRefreshDur.Set(dur)
	i.lastRefreshTime.SetToCurrentTime()

	if err := reg.Register(i.peerScrapeErrors); err != nil {
		// Already registered on a previous refresh — use MustRegister on the
		// health counters instead by wrapping in an unregistered collector.
		_ = err
	}
	_ = reg.Register(i.apiErrors)
	_ = reg.Register(i.lastRefreshTime)
	_ = reg.Register(i.lastRefreshDur)

	// Atomically publish.
	i.registry.store(reg)

	i.peerMu.Lock()
	i.peerCache = newPeerCache
	i.peerMu.Unlock()
}

// registerAPIMetrics adds device-level and tailnet-aggregate metrics to reg.
func (i *integration) registerAPIMetrics(reg *prometheus.Registry, devices []tsclient.Device) error {
	now := time.Now()

	// Descriptor definitions.
	descInfo := prometheus.NewDesc(
		"tailscale_device_info",
		"Information about a device in the tailnet. Always 1.",
		[]string{"name", "id", "os", "tailscale_ip"},
		nil,
	)
	descAuthorized := prometheus.NewDesc(
		"tailscale_device_authorized",
		"Whether the device is authorized on the tailnet (1=authorized, 0=not).",
		[]string{"name", "id"},
		nil,
	)
	descOnline := prometheus.NewDesc(
		"tailscale_device_online",
		"Whether the device has been seen within the last 5 minutes (1=online, 0=offline).",
		[]string{"name", "id"},
		nil,
	)
	descLastSeen := prometheus.NewDesc(
		"tailscale_device_last_seen_seconds",
		"Unix timestamp of when the device was last seen.",
		[]string{"name", "id"},
		nil,
	)
	descExpiry := prometheus.NewDesc(
		"tailscale_device_key_expiry_seconds",
		"Unix timestamp of when the device's key expires. Zero if key expiry is disabled.",
		[]string{"name", "id"},
		nil,
	)
	descUpdateAvailable := prometheus.NewDesc(
		"tailscale_device_update_available",
		"Whether a Tailscale client update is available for the device (1=available, 0=not).",
		[]string{"name", "id"},
		nil,
	)
	descTotal := prometheus.NewDesc(
		"tailscale_devices_total",
		"Total number of devices in the tailnet.",
		nil, nil,
	)
	descOnlineTotal := prometheus.NewDesc(
		"tailscale_devices_online_total",
		"Number of devices seen within the last 5 minutes.",
		nil, nil,
	)
	descAuthorizedTotal := prometheus.NewDesc(
		"tailscale_devices_authorized_total",
		"Number of authorized devices in the tailnet.",
		nil, nil,
	)

	var onlineCount, authorizedCount float64

	collector := &constCollector{
		descs: []*prometheus.Desc{
			descInfo, descAuthorized, descOnline, descLastSeen,
			descExpiry, descUpdateAvailable,
			descTotal, descOnlineTotal, descAuthorizedTotal,
		},
		collect: func(ch chan<- prometheus.Metric) {
			for _, d := range devices {
				ip := ""
				if len(d.Addresses) > 0 {
					// Strip CIDR prefix (e.g. "100.64.1.1/32" → "100.64.1.1").
					ip, _, _ = strings.Cut(d.Addresses[0], "/")
				}

				ch <- prometheus.MustNewConstMetric(descInfo, prometheus.GaugeValue, 1, d.Name, d.ID, d.OS, ip)

				authorized := boolToFloat(d.Authorized)
				ch <- prometheus.MustNewConstMetric(descAuthorized, prometheus.GaugeValue, authorized, d.Name, d.ID)

				online := 0.0
				lastSeen := lastSeenTime(d)
				if !lastSeen.IsZero() && now.Sub(lastSeen) < onlineThreshold {
					online = 1.0
				}
				ch <- prometheus.MustNewConstMetric(descOnline, prometheus.GaugeValue, online, d.Name, d.ID)

				if !lastSeen.IsZero() {
					ch <- prometheus.MustNewConstMetric(descLastSeen, prometheus.GaugeValue, float64(lastSeen.Unix()), d.Name, d.ID)
				}

				expiry := d.Expires.Time
				expiryVal := 0.0
				if !d.KeyExpiryDisabled && !expiry.IsZero() {
					expiryVal = float64(expiry.Unix())
				}
				ch <- prometheus.MustNewConstMetric(descExpiry, prometheus.GaugeValue, expiryVal, d.Name, d.ID)

				ch <- prometheus.MustNewConstMetric(descUpdateAvailable, prometheus.GaugeValue, boolToFloat(d.UpdateAvailable), d.Name, d.ID)
			}

			ch <- prometheus.MustNewConstMetric(descTotal, prometheus.GaugeValue, float64(len(devices)))
			ch <- prometheus.MustNewConstMetric(descOnlineTotal, prometheus.GaugeValue, onlineCount)
			ch <- prometheus.MustNewConstMetric(descAuthorizedTotal, prometheus.GaugeValue, authorizedCount)
		},
	}

	// Pre-compute aggregate counts (before registering the collector).
	for _, d := range devices {
		lastSeen := lastSeenTime(d)
		if !lastSeen.IsZero() && now.Sub(lastSeen) < onlineThreshold {
			onlineCount++
		}
		if d.Authorized {
			authorizedCount++
		}
	}

	return reg.Register(collector)
}

// scrapePeer fetches raw Prometheus text from url using client.
func scrapePeer(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if readErr != nil {
			break
		}
	}
	return buf, nil
}

// copyPeerCache returns a shallow copy of the peer cache map. Entries are
// treated as immutable, so the shared raw/labels are safe to reuse.
func copyPeerCache(m map[string]peerEntry) map[string]peerEntry {
	cp := make(map[string]peerEntry, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// newAPIClient builds a management API client for the configured auth mode.
// With OAuth, the v2 client mints and auto-refreshes tokens via the client-
// credentials flow (the token endpoint is derived from BaseURL); otherwise it
// authenticates with the static API key.
func (i *integration) newAPIClient() (*tsclient.Client, error) {
	c := &tsclient.Client{Tailnet: i.cfg.Tailnet}
	if i.cfg.APIBaseURL != "" {
		u, err := url.Parse(i.cfg.APIBaseURL)
		if err != nil {
			return nil, fmt.Errorf("parse api_base_url %q: %w", i.cfg.APIBaseURL, err)
		}
		c.BaseURL = u
	}
	if i.cfg.OAuthClientID != "" {
		c.Auth = &tsclient.OAuth{
			ClientID:     i.cfg.OAuthClientID,
			ClientSecret: i.cfg.OAuthClientSecret,
		}
	} else {
		c.APIKey = i.cfg.APIKey
	}
	return c, nil
}

// resolveTarget picks the metrics (port, path) and any extra labels for a
// device based on its tags. It returns ok=false when no configured target
// group matches — the device is then skipped. When no targets are configured,
// the legacy PeerMetricsPort and PeerMetricsPath apply to every device.
func (i *integration) resolveTarget(tags []string) (port int, path string, labels map[string]string, ok bool) {
	if len(i.cfg.Targets) == 0 {
		return i.cfg.PeerMetricsPort, i.cfg.PeerMetricsPath, nil, true
	}
	for _, t := range i.cfg.Targets {
		if len(t.MatchTags) == 0 || anyTagMatches(t.MatchTags, tags) {
			path := t.Path
			if path == "" {
				path = defaultPeerMetricsPath
			}
			return t.Port, path, t.Labels, true
		}
	}
	return 0, "", nil, false
}

// anyTagMatches reports whether any device tag matches any glob pattern
// (e.g. "tag:*-proxy" matches "tag:us-east-proxy").
func anyTagMatches(patterns, tags []string) bool {
	for _, p := range patterns {
		for _, tag := range tags {
			if matched, _ := filepath.Match(p, tag); matched {
				return true
			}
		}
	}
	return false
}

// boolToFloat converts a bool to 0.0 or 1.0.
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// lastSeenTime returns the device's last-seen time, or the zero time if the
// API omitted it (LastSeen is nil when the device is connected to control).
func lastSeenTime(d tsclient.Device) time.Time {
	if d.LastSeen == nil {
		return time.Time{}
	}
	return d.LastSeen.Time
}

// constCollector is a prometheus.Collector that calls a collect function and
// declares a fixed set of descriptors.
type constCollector struct {
	descs   []*prometheus.Desc
	collect func(chan<- prometheus.Metric)
}

func (c *constCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descs {
		ch <- d
	}
}

func (c *constCollector) Collect(ch chan<- prometheus.Metric) {
	c.collect(ch)
}
