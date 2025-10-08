// Package elasticsearch_exporter instantiates the exporter from github.com/justwatchcom/elasticsearch_exporter - replaced for github.com/prometheus-community/elasticsearch_exporter
// Using the YAML config provided by the agent
package elasticsearch_exporter

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	promCfg "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"

	"github.com/prometheus-community/elasticsearch_exporter/collector"
	"github.com/prometheus-community/elasticsearch_exporter/pkg/clusterinfo"
)

// DefaultConfig holds the default settings for the elasticsearch_exporter
// integration.
var DefaultConfig = Config{
	Address:                   "http://localhost:9200",
	Timeout:                   5 * time.Second,
	Node:                      "_local",
	ExportClusterInfoInterval: 5 * time.Minute,
	IncludeAliases:            true,
}

// Config controls the elasticsearch_exporter integration.
type Config struct {
	// HTTP API address of an Elasticsearch node.
	Address string `yaml:"address,omitempty"`
	// Timeout for trying to get stats from Elasticsearch.
	Timeout time.Duration `yaml:"timeout,omitempty"`
	// Export stats for all nodes in the cluster. If used, this flag will override the flag es.node.
	AllNodes bool `yaml:"all,omitempty"`
	// Node's name of which metrics should be exposed.
	Node string `yaml:"node,omitempty"`
	// Export stats for indices in the cluster.
	ExportIndices bool `yaml:"indices,omitempty"`
	// Export stats for settings of all indices of the cluster.
	ExportIndicesSettings bool `yaml:"indices_settings,omitempty"`
	// Export stats for cluster settings.
	ExportClusterSettings bool `yaml:"cluster_settings,omitempty"`
	// Export stats for shards in the cluster (implies indices).
	ExportShards bool `yaml:"shards,omitempty"`
	// Include informational aliases metrics
	IncludeAliases bool `yaml:"aliases,omitempty"`
	// Export stats for the cluster snapshots.
	ExportSnapshots bool `yaml:"snapshots,omitempty"`
	// Cluster info update interval for the cluster label.
	ExportClusterInfoInterval time.Duration `yaml:"clusterinfo_interval,omitempty"`
	// Path to PEM file that contains trusted Certificate Authorities for the Elasticsearch connection.
	CA string `yaml:"ca,omitempty"`
	// Path to PEM file that contains the private key for client auth when connecting to Elasticsearch.
	ClientPrivateKey string `yaml:"client_private_key,omitempty"`
	// Path to PEM file that contains the corresponding cert for the private key to connect to Elasticsearch.
	ClientCert string `yaml:"client_cert,omitempty"`
	// Skip SSL verification when connecting to Elasticsearch.
	InsecureSkipVerify bool `yaml:"ssl_skip_verify,omitempty"`
	// Export stats for Data Streams
	ExportDataStreams bool `yaml:"data_stream,omitempty"`
	// Export stats for Snapshot Lifecycle Management
	ExportSLM bool `yaml:"slm,omitempty"`
	// BasicAuth block allows secure connection with Elasticsearch cluster via Basic-Auth
	BasicAuth *promCfg.BasicAuth `yaml:"basic_auth,omitempty"`
}

// Custom http.Transport struct for Basic Auth-secured communication with ES cluster
type BasicAuthHTTPTransport struct {
	http.Transport
	authHeader string
}

func (b *BasicAuthHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if b.authHeader != "" {
		req.Header.Add("authorization", b.authHeader)
	}
	return b.Transport.RoundTrip(req)
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "elasticsearch_exporter"
}

// InstanceKey returns the hostname:port of the elasticsearch node being queried.
func (c *Config) InstanceKey(_ string) (string, error) {
	u, err := url.Parse(c.Address)
	if err != nil {
		return "", fmt.Errorf("could not parse url: %w", err)
	}
	return u.Host, nil
}

// NewIntegration creates a new elasticsearch_exporter
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("elasticsearch"))
}

// New creates a new elasticsearch_exporter
// This function replicates the main() function of github.com/justwatchcom/elasticsearch_exporter
// but uses yaml configuration instead of kingpin flags.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	if c.Address == "" {
		return nil, fmt.Errorf("empty elasticsearch_address provided")
	}
	esURL, err := url.Parse(c.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse elasticsearch_address: %w", err)
	}

	// returns nil if not provided and falls back to simple TCP.
	tlsConfig := createTLSConfig(c.CA, c.ClientCert, c.ClientPrivateKey, c.InsecureSkipVerify)

	esHttpTransport := &BasicAuthHTTPTransport{
		Transport: http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
		},
	}

	if c.BasicAuth != nil {
		password := string(c.BasicAuth.Password)
		if len(c.BasicAuth.PasswordFile) > 0 {
			buff, err := os.ReadFile(c.BasicAuth.PasswordFile)
			if err != nil {
				return nil, fmt.Errorf("unable to load password file %s: %w", c.BasicAuth.PasswordFile, err)
			}
			password = strings.TrimSpace(string(buff))
		}
		username := c.BasicAuth.Username
		if len(c.BasicAuth.UsernameFile) > 0 {
			buff, err := os.ReadFile(c.BasicAuth.UsernameFile)
			if err != nil {
				return nil, fmt.Errorf("unable to load username file %s: %w", c.BasicAuth.UsernameFile, err)
			}
			username = strings.TrimSpace(string(buff))
		}
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		esHttpTransport.authHeader = "Basic " + encodedAuth
	}

	httpClient := &http.Client{
		Timeout:   c.Timeout,
		Transport: esHttpTransport,
	}

	clusterInfoRetriever := clusterinfo.New(logger, httpClient, esURL, c.ExportClusterInfoInterval)

	collectors := []prometheus.Collector{
		clusterInfoRetriever,
		collector.NewClusterHealth(logger, httpClient, esURL),
		collector.NewNodes(logger, httpClient, esURL, c.AllNodes, c.Node),
	}

	if c.ExportIndices || c.ExportShards {
		iC := collector.NewIndices(logger, httpClient, esURL, c.ExportShards, c.IncludeAliases)
		collectors = append(collectors, iC)
		if registerErr := clusterInfoRetriever.RegisterConsumer(iC); registerErr != nil {
			return nil, fmt.Errorf("failed to register indices collector in cluster info: %w", err)
		}
	}

	if c.ExportSnapshots {
		collectors = append(collectors, collector.NewSnapshots(logger, httpClient, esURL))
	}

	if c.ExportClusterSettings {
		collectors = append(collectors, collector.NewClusterSettings(logger, httpClient, esURL))
	}

	if c.ExportDataStreams {
		collectors = append(collectors, collector.NewDataStream(logger, httpClient, esURL))
	}

	if c.ExportIndicesSettings {
		collectors = append(collectors, collector.NewIndicesSettings(logger, httpClient, esURL))
	}

	if c.ExportSLM {
		collectors = append(collectors, collector.NewSLM(logger, httpClient, esURL))
	}

	start := func(ctx context.Context) error {
		// start the cluster info retriever
		switch runErr := clusterInfoRetriever.Run(ctx); runErr {
		case nil:
			level.Info(logger).Log(
				"msg", "started cluster info retriever",
				"interval", c.ExportClusterInfoInterval.String(),
			)
		case clusterinfo.ErrInitialCallTimeout:
			level.Info(logger).Log("msg", "initial cluster info call timed out")
		default:
			level.Error(logger).Log("msg", "failed to run cluster info retriever", "err", err)
			return err
		}

		// Wait until we're done
		<-ctx.Done()
		return ctx.Err()
	}

	return integrations.NewCollectorIntegration(c.Name(),
		integrations.WithCollectors(collectors...),
		integrations.WithRunner(start),
	), nil
}
