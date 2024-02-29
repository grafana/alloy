package config

import (
	"bytes"
	"text/template"

	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/loki/clients/pkg/promtail/scrapeconfig"
	"github.com/prometheus/common/model"
	pc "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"gopkg.in/yaml.v2"
)

type (
	RemoteConfig struct {
		BaseConfig    BaseConfigContent `json:"base_config" yaml:"base_config"`
		Snippets      []Snippet         `json:"snippets" yaml:"snippets"`
		AgentMetadata AgentMetadata     `json:"agent_metadata,omitempty" yaml:"agent_metadata,omitempty"`
	}

	// BaseConfigContent is the content of a base config
	BaseConfigContent string

	// Snippet is a snippet of configuration returned by the config API.
	Snippet struct {
		// Config is the snippet of config to be included.
		Config string `json:"config" yaml:"config"`
	}

	AgentMetadata struct {
		ExternalLabels    map[string]string `json:"external_labels,omitempty" yaml:"external_labels,omitempty"`
		TemplateVariables map[string]any    `json:"template_variables,omitempty" yaml:"template_variables,omitempty"`
	}

	// SnippetContent defines the internal structure of a snippet configuration.
	SnippetContent struct {
		// MetricsScrapeConfigs is a YAML containing list of metrics scrape configs.
		MetricsScrapeConfigs []*pc.ScrapeConfig `yaml:"metrics_scrape_configs,omitempty"`

		// LogsScrapeConfigs is a YAML containing list of logs scrape configs.
		LogsScrapeConfigs []scrapeconfig.Config `yaml:"logs_scrape_configs,omitempty"`

		// IntegrationConfigs is a YAML containing list of integrations.
		IntegrationConfigs integrations.ManagerConfig `yaml:"integration_configs,omitempty"`
	}
)

func NewRemoteConfig(buf []byte) (*RemoteConfig, error) {
	rc := &RemoteConfig{}
	err := yaml.Unmarshal(buf, rc)
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// BuildAgentConfig builds an agent configuration from a base config and a list of snippets
func (rc *RemoteConfig) BuildAgentConfig() (*Config, error) {
	baseConfig, err := evaluateTemplate(string(rc.BaseConfig), rc.AgentMetadata.TemplateVariables)
	if err != nil {
		return nil, err
	}

	c := DefaultConfig()
	err = yaml.Unmarshal([]byte(baseConfig), &c)
	if err != nil {
		return nil, err
	}

	// For now Agent Management only supports integrations v1
	if err := c.Integrations.setVersion(IntegrationsVersion1); err != nil {
		return nil, err
	}

	err = appendSnippets(&c, rc.Snippets, rc.AgentMetadata.TemplateVariables)
	if err != nil {
		return nil, err
	}
	appendExternalLabels(&c, rc.AgentMetadata.ExternalLabels)
	return &c, nil
}

func appendSnippets(c *Config, snippets []Snippet, templateVars map[string]any) error {
	metricsConfigs := instance.DefaultConfig
	metricsConfigs.Name = "snippets"
	logsConfigs := logs.InstanceConfig{
		Name:         "snippets",
		ScrapeConfig: []scrapeconfig.Config{},
	}
	logsConfigs.Initialize()
	integrationConfigs := integrations.DefaultManagerConfig()

	// Map used to identify if an integration is already configured and avoid overriding it
	configuredIntegrations := map[string]bool{}
	for _, itg := range c.Integrations.ConfigV1.Integrations {
		configuredIntegrations[itg.Name()] = true
	}

	for _, snippet := range snippets {
		snippetConfig, err := evaluateTemplate(snippet.Config, templateVars)
		if err != nil {
			return err
		}

		var snippetContent SnippetContent
		err = yaml.Unmarshal([]byte(snippetConfig), &snippetContent)
		if err != nil {
			return err
		}
		metricsConfigs.ScrapeConfigs = append(metricsConfigs.ScrapeConfigs, snippetContent.MetricsScrapeConfigs...)
		logsConfigs.ScrapeConfig = append(logsConfigs.ScrapeConfig, snippetContent.LogsScrapeConfigs...)

		for _, snip := range snippetContent.IntegrationConfigs.Integrations {
			if _, ok := configuredIntegrations[snip.Name()]; !ok {
				integrationConfigs.Integrations = append(integrationConfigs.Integrations, snip)
				configuredIntegrations[snip.Name()] = true
			}
		}
	}

	if len(metricsConfigs.ScrapeConfigs) > 0 {
		c.Metrics.Configs = append(c.Metrics.Configs, metricsConfigs)
	}

	if len(logsConfigs.ScrapeConfig) > 0 {
		// rc.Config.Logs is initialized as nil, so we need to check if it's nil before appending
		if c.Logs == nil {
			c.Logs = &logs.Config{
				Configs: []*logs.InstanceConfig{},
			}
		}
		c.Logs.Configs = append(c.Logs.Configs, &logsConfigs)
	}

	c.Integrations.ConfigV1.Integrations = append(c.Integrations.ConfigV1.Integrations, integrationConfigs.Integrations...)
	return nil
}

func appendExternalLabels(c *Config, externalLabels map[string]string) {
	// Avoid doing anything if there are no external labels
	if len(externalLabels) == 0 {
		return
	}
	// Start off with the existing external labels, which will only be added to (not replaced)
	metricsExternalLabels := c.Metrics.Global.Prometheus.ExternalLabels.Map()
	for k, v := range externalLabels {
		if _, ok := metricsExternalLabels[k]; !ok {
			metricsExternalLabels[k] = v
		}
	}

	logsExternalLabels := make(model.LabelSet)
	for k, v := range externalLabels {
		logsExternalLabels[model.LabelName(k)] = model.LabelValue(v)
	}

	c.Metrics.Global.Prometheus.ExternalLabels = labels.FromMap(metricsExternalLabels)
	for i, cc := range c.Logs.Global.ClientConfigs {
		c.Logs.Global.ClientConfigs[i].ExternalLabels.LabelSet = logsExternalLabels.Merge(cc.ExternalLabels.LabelSet)
	}
}

func evaluateTemplate(config string, templateVariables map[string]any) (string, error) {
	tpl, err := template.New("config").Parse(config)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tpl.Execute(&buf, templateVariables)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
