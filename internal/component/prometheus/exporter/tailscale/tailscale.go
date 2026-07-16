package tailscale

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// against the Tailscale management API. Mutually exclusive with api_key_file
	// and the oauth block.
	APIKey alloytypes.Secret `alloy:"api_key,attr,optional"`

	// APIKeyFile reads the API key from a file instead of inline. Useful for
	// file-mounted / workload-provided secrets.
	APIKeyFile string `alloy:"api_key_file,attr,optional"`

	// AuthKey is the Tailscale auth key (tskey-auth-...) used to join the
	// tailnet as an embedded node via tsnet. Required with API-key auth; not
	// used with the oauth block (tsnet mints its own key via OAuth).
	AuthKey alloytypes.Secret `alloy:"auth_key,attr,optional"`

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

	// OAuth authenticates via a Tailscale OAuth client instead of api_key +
	// auth_key. When set, it authenticates the management API and the tsnet node
	// join. Mutually exclusive with api_key/api_key_file.
	OAuth *OAuthArguments `alloy:"oauth,block,optional"`

	// Targets maps node types to the port/path where they expose metrics. Use
	// this when different node types listen on different ports — for example,
	// clients and exit nodes on 5252, but Tailscale Kubernetes operator proxies
	// and ingresses on 9002. When no target blocks are given, every node is
	// scraped on peer_metrics_port/peer_metrics_path.
	Targets []Target `alloy:"target,block,optional"`
}

// OAuthArguments configures OAuth-client authentication.
type OAuthArguments struct {
	// ClientID is the Tailscale OAuth client ID.
	ClientID string `alloy:"client_id,attr"`

	// ClientSecret is the OAuth client secret. Mutually exclusive with
	// client_secret_file; exactly one is required.
	ClientSecret alloytypes.Secret `alloy:"client_secret,attr,optional"`

	// ClientSecretFile reads the OAuth client secret from a file.
	ClientSecretFile string `alloy:"client_secret_file,attr,optional"`

	// AdvertiseTags are applied to the embedded tsnet node. Required, because
	// OAuth-generated auth keys must be tagged, and the tags must be owned by
	// the OAuth client in the tailnet policy.
	AdvertiseTags []string `alloy:"advertise_tags,attr,optional"`
}

// Target maps a group of nodes (matched by tag) to a metrics port and path.
type Target struct {
	// MatchTags is a list of tag glob patterns (e.g. "tag:*-proxy"). A node
	// matches if any of its tags matches any pattern. An empty list matches
	// every node, making the block a catch-all — list it last.
	MatchTags []string `alloy:"match_tags,attr,optional"`

	// Port on the matched nodes where Tailscale metrics are exposed.
	Port int `alloy:"port,attr"`

	// Path on the matched nodes' metrics endpoint. Defaults to "/metrics".
	Path string `alloy:"path,attr,optional"`

	// Labels are extra labels attached to every metric from matched nodes.
	Labels map[string]string `alloy:"labels,attr,optional"`
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

	hasKey := a.APIKey != "" || a.APIKeyFile != ""
	hasOAuth := a.OAuth != nil
	switch {
	case hasKey && hasOAuth:
		return fmt.Errorf("api_key/api_key_file and an oauth block are mutually exclusive")
	case !hasKey && !hasOAuth:
		return fmt.Errorf("one of api_key, api_key_file, or an oauth block is required")
	case hasKey:
		if a.APIKey != "" && a.APIKeyFile != "" {
			return fmt.Errorf("api_key and api_key_file are mutually exclusive")
		}
		if a.AuthKey == "" {
			return fmt.Errorf("auth_key is required when authenticating with an API key")
		}
	case hasOAuth:
		if a.OAuth.ClientID == "" {
			return fmt.Errorf("oauth.client_id is required")
		}
		hasSecret := a.OAuth.ClientSecret != "" || a.OAuth.ClientSecretFile != ""
		if !hasSecret {
			return fmt.Errorf("oauth.client_secret or oauth.client_secret_file is required")
		}
		if a.OAuth.ClientSecret != "" && a.OAuth.ClientSecretFile != "" {
			return fmt.Errorf("oauth.client_secret and oauth.client_secret_file are mutually exclusive")
		}
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
	for idx, t := range a.Targets {
		if t.Port <= 0 || t.Port > 65535 {
			return fmt.Errorf("target[%d].port must be between 1 and 65535", idx)
		}
	}
	return nil
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)

	stateDir := a.StateDir
	if stateDir == "" {
		stateDir = filepath.Join(opts.DataPath, "tsnet")
	}

	var targets []tailscale_exporter.ScrapeTarget
	for _, t := range a.Targets {
		targets = append(targets, tailscale_exporter.ScrapeTarget{
			MatchTags: t.MatchTags,
			Port:      t.Port,
			Path:      t.Path,
			Labels:    t.Labels,
		})
	}

	apiKey := string(a.APIKey)
	if a.APIKeyFile != "" {
		b, err := os.ReadFile(a.APIKeyFile)
		if err != nil {
			return nil, "", fmt.Errorf("reading api_key_file: %w", err)
		}
		apiKey = strings.TrimSpace(string(b))
	}

	var oauthID, oauthSecret string
	var advertiseTags []string
	if a.OAuth != nil {
		oauthID = a.OAuth.ClientID
		oauthSecret = string(a.OAuth.ClientSecret)
		if a.OAuth.ClientSecretFile != "" {
			b, err := os.ReadFile(a.OAuth.ClientSecretFile)
			if err != nil {
				return nil, "", fmt.Errorf("reading oauth client_secret_file: %w", err)
			}
			oauthSecret = strings.TrimSpace(string(b))
		}
		advertiseTags = a.OAuth.AdvertiseTags
	}

	cfg := tailscale_exporter.Config{
		Tailnet:           a.Tailnet,
		APIKey:            apiKey,
		AuthKey:           string(a.AuthKey),
		OAuthClientID:     oauthID,
		OAuthClientSecret: oauthSecret,
		AdvertiseTags:     advertiseTags,
		APIBaseURL:        a.APIBaseURL,
		StateDir:          stateDir,
		TSNetHostname:     a.TSNetHostname,
		RefreshInterval:   a.RefreshInterval,
		PeerMetricsPort:   a.PeerMetricsPort,
		PeerMetricsPath:   a.PeerMetricsPath,
		PeerScrapeTimeout: a.PeerScrapeTimeout,
		Targets:           targets,
	}

	integration, err := tailscale_exporter.New(opts.Logger, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("creating tailscale integration: %w", err)
	}

	// Use the tailnet name as the instance key — it's stable and descriptive.
	return integration, a.Tailnet, nil
}
