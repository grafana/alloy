package github_exporter

import (
	"fmt"
	"net/url"

	gh_config "github.com/githubexporter/github-exporter/config"
	"github.com/githubexporter/github-exporter/exporter"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	config_util "github.com/prometheus/common/config"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

// DefaultConfig holds the default settings for the github_exporter integration
var DefaultConfig = Config{
	APIURL: "https://api.github.com",
}

// Config controls github_exporter
type Config struct {
	// URL for the GitHub API
	APIURL string `yaml:"api_url,omitempty"`

	// A list of GitHub repositories for which to collect metrics.
	Repositories []string `yaml:"repositories,omitempty"`

	// A list of GitHub organizations for which to collect metrics.
	Organizations []string `yaml:"organizations,omitempty"`

	// A list of GitHub users for which to collect metrics.
	Users []string `yaml:"users,omitempty"`

	// A GitHub authentication token that allows the API to be queried more often.
	APIToken config_util.Secret `yaml:"api_token,omitempty"`

	// A path to a file containing a GitHub authentication token that allows the API to be queried more often. If supplied, this supersedes `api_token`
	APITokenFile string `yaml:"api_token_file,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return "github_exporter"
}

// InstanceKey returns the hostname:port of the GitHub API server.
func (c *Config) InstanceKey(_ string) (string, error) {
	u, err := url.Parse(c.APIURL)
	if err != nil {
		return "", fmt.Errorf("could not parse url: %w", err)
	}
	if u.Port() != "" {
		return u.Host + ":" + u.Port(), nil
	}
	return u.Host, nil
}

// NewIntegration creates a new github_exporter
func (c *Config) NewIntegration(logger log.Logger) (integrations.Integration, error) {
	return New(logger, c)
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("github"))
}

// New creates a new github_exporter integration.
func New(logger log.Logger, c *Config) (integrations.Integration, error) {
	conf := gh_config.Config{}
	err := conf.SetAPIURL(c.APIURL)
	if err != nil {
		level.Error(logger).Log("msg", "api url is invalid", "err", err)
		return nil, err
	}
	conf.SetRepositories(c.Repositories)
	conf.SetOrganisations(c.Organizations)
	conf.SetUsers(c.Users)
	if c.APIToken != "" {
		conf.SetAPIToken(string(c.APIToken))
	}
	if c.APITokenFile != "" {
		err = conf.SetAPITokenFromFile(c.APITokenFile)
		if err != nil {
			level.Error(logger).Log("msg", "unable to load GitHub API token from file", "err", err)
			return nil, err
		}
	}

	ghExporter := exporter.Exporter{
		APIMetrics: exporter.AddMetrics(),
		Config:     conf,
	}

	return integrations.NewCollectorIntegration(
		c.Name(),
		integrations.WithCollectors(&ghExporter),
	), nil
}
