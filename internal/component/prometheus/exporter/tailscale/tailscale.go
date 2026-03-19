package tailscale

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/tailscale_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.tailscale",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "tailscale"),
	})
}

// DefaultArguments holds non-zero defaults for Arguments.
var DefaultArguments = Arguments{
	APIBaseURL:        "https://api.tailscale.com",
	RefreshInterval:   60 * time.Second,
	PeerMetricsPort:   5252,
	PeerMetricsPath:   "/metrics",
	PeerScrapeTimeout: 3 * time.Second,
	TSNetHostname:     "alloy-tailscale-exporter",
}

// Arguments configures the prometheus.exporter.tailscale component.
type Arguments struct {
	// Tailnet is the tailnet name to monitor (e.g. "example.com").
	Tailnet string `alloy:"tailnet,attr"`

	// APIKey is the Tailscale API key (tskey-api-...) used to authenticate
	// against the Tailscale management API.
	APIKey alloytypes.Secret `alloy:"api_key,attr"`

	// AuthKey is the Tailscale auth key (tskey-auth-...) used to join the
	// tailnet as an embedded node via tsnet. The node persists its credentials
	// in StateDir after the first join, so the key is only consumed once.
	AuthKey alloytypes.Secret `alloy:"auth_key,attr"`

	// StateDir is the directory where the embedded tsnet node stores its state
	// (WireGuard keys, certificates). Defaults to a subdirectory of the
	// component's data path. Must be on persistent storage to avoid
	// re-authentication on every restart.
	StateDir string `alloy:"state_dir,attr,optional"`

	// TSNetHostname is the hostname this node presents to the tailnet.
	TSNetHostname string `alloy:"tsnet_hostname,attr,optional"`

	// APIBaseURL overrides the default Tailscale management API URL.
	APIBaseURL string `alloy:"api_base_url,attr,optional"`

	// RefreshInterval controls how often the exporter polls the API and
	// scrapes peer metrics.
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`

	// PeerMetricsPort is the port on each peer where the Tailscale daemon
	// exposes Prometheus metrics. Defaults to 5252.
	PeerMetricsPort int `alloy:"peer_metrics_port,attr,optional"`

	// PeerMetricsPath is the HTTP path on each peer's metrics endpoint.
	PeerMetricsPath string `alloy:"peer_metrics_path,attr,optional"`

	// PeerScrapeTimeout is the per-peer HTTP timeout when scraping metrics.
	// Peers that don't expose metrics will fail fast on connection refused;
	// this timeout only applies to peers that accept the connection but don't
	// respond (e.g. firewalled).
	PeerScrapeTimeout time.Duration `alloy:"peer_scrape_timeout,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.Tailnet == "" {
		return fmt.Errorf("tailnet is required")
	}
	if a.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if a.AuthKey == "" {
		return fmt.Errorf("auth_key is required")
	}
	if a.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be positive")
	}
	if a.PeerMetricsPort <= 0 || a.PeerMetricsPort > 65535 {
		return fmt.Errorf("peer_metrics_port must be between 1 and 65535")
	}
	if a.PeerScrapeTimeout <= 0 {
		return fmt.Errorf("peer_scrape_timeout must be positive")
	}
	return nil
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)

	stateDir := a.StateDir
	if stateDir == "" {
		stateDir = filepath.Join(opts.DataPath, "tsnet")
	}

	cfg := tailscale_exporter.Config{
		Tailnet:           a.Tailnet,
		APIKey:            string(a.APIKey),
		AuthKey:           string(a.AuthKey),
		APIBaseURL:        a.APIBaseURL,
		StateDir:          stateDir,
		TSNetHostname:     a.TSNetHostname,
		RefreshInterval:   a.RefreshInterval,
		PeerMetricsPort:   a.PeerMetricsPort,
		PeerMetricsPath:   a.PeerMetricsPath,
		PeerScrapeTimeout: a.PeerScrapeTimeout,
	}

	integration, err := tailscale_exporter.New(opts.Logger, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("creating tailscale integration: %w", err)
	}

	// Use the tailnet name as the instance key — it's stable and descriptive.
	return integration, a.Tailnet, nil
}
