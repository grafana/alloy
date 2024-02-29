package config

import (
	"testing"
	"time"

	process_exporter "github.com/grafana/agent/internal/static/integrations/process_exporter"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestBuildRemoteConfig(t *testing.T) {
	baseConfig := `
server:
    log_level: debug
`
	metricsSnippets := []Snippet{{
		Config: `
metrics_scrape_configs:
  - job_name: 'prometheus'
    scrape_interval: 15s
    static_configs:
    - targets: ['localhost:9090']
`,
	}}
	logsSnippets := []Snippet{{
		Config: `
logs_scrape_configs:
  - job_name: 'loki'
    static_configs:
    - targets: ['localhost:3100']
`,
	}}
	integrationSnippets := []Snippet{{
		Config: `
integration_configs:
  agent:
    enabled: true
    relabel_configs:
      - action: replace
        source_labels:
          - agent_hostname
        target_label: instance
`,
	}}

	allSnippets := []Snippet{}
	allSnippets = append(allSnippets, metricsSnippets...)
	allSnippets = append(allSnippets, logsSnippets...)
	allSnippets = append(allSnippets, integrationSnippets...)

	t.Run("only metrics snippets provided", func(t *testing.T) {
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   metricsSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, len(c.Metrics.Configs), 1)
		require.Empty(t, c.Logs)
		require.Empty(t, c.Integrations.ConfigV1.Integrations)
	})

	t.Run("only log snippets provided", func(t *testing.T) {
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   logsSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, len(c.Logs.Configs), 1)
		require.Empty(t, c.Metrics.Configs)
		require.Empty(t, c.Integrations.ConfigV1.Integrations)
	})

	t.Run("only integration snippets provided", func(t *testing.T) {
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   integrationSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Empty(t, c.Metrics.Configs)
		require.Empty(t, c.Logs)
		require.Equal(t, 1, len(c.Integrations.ConfigV1.Integrations))
	})

	t.Run("base with already logs, metrics and integrations provided", func(t *testing.T) {
		fullConfig := `
metrics:
  configs:
  - name: default
    scrape_configs:
    - job_name: default-prom
      static_configs:
      - targets:
        - localhost:9090
logs:
  positions_directory: /tmp/grafana-agent-positions
  configs:
  - name: default
    scrape_configs:
    - job_name: default-loki
      static_configs:
      - targets:
        - localhost:3100
integrations:
  node_exporter:
    enabled: true
`
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(fullConfig),
			Snippets:   allSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, len(c.Logs.Configs), 2)
		require.Equal(t, len(c.Metrics.Configs), 2)
		require.Equal(t, 2, len(c.Integrations.ConfigV1.Integrations))
	})

	t.Run("do not override integrations defined in base config with the ones defined in snippets", func(t *testing.T) {
		baseConfig := `
integrations:
  node_exporter:
    enabled: false
`

		snippets := []Snippet{{
			Config: `
integration_configs:
  node_exporter:
    enabled: true`,
		}}

		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   snippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Integrations.ConfigV1.Integrations))
		require.False(t, c.Integrations.ConfigV1.Integrations[0].Common.Enabled)
	})

	t.Run("all snippets provided", func(t *testing.T) {
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   allSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Logs.Configs))
		require.Equal(t, 1, len(c.Metrics.Configs))
		require.Equal(t, 1, len(c.Integrations.ConfigV1.Integrations))

		// check some fields to make sure the config was parsed correctly
		require.Equal(t, "prometheus", c.Metrics.Configs[0].ScrapeConfigs[0].JobName)
		require.Equal(t, "loki", c.Logs.Configs[0].ScrapeConfig[0].JobName)
		require.Equal(t, "agent", c.Integrations.ConfigV1.Integrations[0].Name())

		// make sure defaults for metric snippets are applied
		require.Equal(t, instance.DefaultConfig.WALTruncateFrequency, c.Metrics.Configs[0].WALTruncateFrequency)
		require.Equal(t, instance.DefaultConfig.HostFilter, c.Metrics.Configs[0].HostFilter)
		require.Equal(t, instance.DefaultConfig.MinWALTime, c.Metrics.Configs[0].MinWALTime)
		require.Equal(t, instance.DefaultConfig.MaxWALTime, c.Metrics.Configs[0].MaxWALTime)
		require.Equal(t, instance.DefaultConfig.RemoteFlushDeadline, c.Metrics.Configs[0].RemoteFlushDeadline)
		require.Equal(t, instance.DefaultConfig.WriteStaleOnShutdown, c.Metrics.Configs[0].WriteStaleOnShutdown)
		require.Equal(t, instance.DefaultGlobalConfig, c.Metrics.Global)

		// make sure defaults for log snippets are applied
		require.Equal(t, 10*time.Second, c.Logs.Configs[0].PositionsConfig.SyncPeriod)
		require.Equal(t, "", c.Logs.Configs[0].PositionsConfig.PositionsFile)
		require.Equal(t, false, c.Logs.Configs[0].PositionsConfig.IgnoreInvalidYaml)
		require.Equal(t, false, c.Logs.Configs[0].TargetConfig.Stdin)

		// make sure defaults for integration snippets are applied
		require.Equal(t, true, c.Integrations.ConfigV1.ScrapeIntegrations)
		require.Equal(t, true, c.Integrations.ConfigV1.UseHostnameLabel)
		require.Equal(t, true, c.Integrations.ConfigV1.ReplaceInstanceLabel)
		require.Equal(t, 5*time.Second, c.Integrations.ConfigV1.IntegrationRestartBackoff)
	})

	t.Run("template variables provided", func(t *testing.T) {
		baseConfig := `
server:
    log_level: {{.log_level}}
`
		templateInsideTemplate := "`{{ .template_inside_template }}`"
		snippet := Snippet{
			Config: `
integration_configs:
  process_exporter:
    enabled: true
    process_names:
      - name: "grafana-agent"
        cmdline:
          - 'grafana-agent'
      - name: "{{.nonexistent.foo.bar.baz.bat}}"
        cmdline:
          - "{{ ` + templateInsideTemplate + ` }}"
      # Custom process monitors
      {{- range $key, $value := .process_exporter_processes }}
      - name: "{{ $value.name }}"
        cmdline:
          - "{{ $value.cmdline }}"
        {{if $value.exe}}
        exe:
          - "{{ $value.exe }}"
        {{end}}
      {{- end }}
`,
		}

		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   []Snippet{snippet},
			AgentMetadata: AgentMetadata{
				TemplateVariables: map[string]any{
					"log_level": "debug",
					"process_exporter_processes": []map[string]string{
						{
							"name":    "java_processes",
							"cmdline": ".*/java",
						},
						{
							"name":    "{{.ExeFull}}:{{.Matches.Cfgfile}}",
							"cmdline": `-config.path\\s+(?P<Cfgfile>\\S+)`,
							"exe":     "/usr/local/bin/process-exporter",
						},
					},
				},
			},
		}

		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Integrations.ConfigV1.Integrations))
		processExporterConfig := c.Integrations.ConfigV1.Integrations[0].Config.(*process_exporter.Config)

		require.Equal(t, 4, len(processExporterConfig.ProcessExporter))

		require.Equal(t, "grafana-agent", processExporterConfig.ProcessExporter[0].Name)
		require.Equal(t, "grafana-agent", processExporterConfig.ProcessExporter[0].CmdlineRules[0])
		require.Equal(t, 0, len(processExporterConfig.ProcessExporter[0].ExeRules))

		require.Equal(t, "<no value>", processExporterConfig.ProcessExporter[1].Name)
		require.Equal(t, "{{ .template_inside_template }}", processExporterConfig.ProcessExporter[1].CmdlineRules[0])
		require.Equal(t, 0, len(processExporterConfig.ProcessExporter[1].ExeRules))

		require.Equal(t, "java_processes", processExporterConfig.ProcessExporter[2].Name)
		require.Equal(t, ".*/java", processExporterConfig.ProcessExporter[2].CmdlineRules[0])
		require.Equal(t, 0, len(processExporterConfig.ProcessExporter[2].ExeRules))

		require.Equal(t, "{{.ExeFull}}:{{.Matches.Cfgfile}}", processExporterConfig.ProcessExporter[3].Name)
		require.Equal(t, `-config.path\s+(?P<Cfgfile>\S+)`, processExporterConfig.ProcessExporter[3].CmdlineRules[0])
		require.Equal(t, "/usr/local/bin/process-exporter", processExporterConfig.ProcessExporter[3].ExeRules[0])
	})

	t.Run("no external labels provided", func(t *testing.T) {
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   allSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Logs.Configs))
		require.Empty(t, c.Metrics.Global.Prometheus.ExternalLabels)
	})

	t.Run("no external labels provided in remote config", func(t *testing.T) {
		baseConfig := `
server:
    log_level: debug
metrics:
    global:
        external_labels:
            foo: bar
logs:
    global:
        clients:
        - external_labels:
            foo: bar
`
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   allSnippets,
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Logs.Configs))
		require.Equal(t, 1, len(c.Logs.Global.ClientConfigs))
		require.Equal(t, c.Logs.Global.ClientConfigs[0].ExternalLabels.LabelSet, model.LabelSet{"foo": "bar"})
		require.Equal(t, 1, len(c.Metrics.Global.Prometheus.ExternalLabels))
		require.Contains(t, c.Metrics.Global.Prometheus.ExternalLabels, labels.Label{Name: "foo", Value: "bar"})
	})

	t.Run("external labels provided", func(t *testing.T) {
		baseConfig := `
server:
    log_level: debug
metrics:
    global:
        remote_write:
        - url: http://localhost:9090/api/prom/push
logs:
    global:
        clients:
        - url: http://localhost:3100/loki/api/v1/push
`

		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   allSnippets,
			AgentMetadata: AgentMetadata{
				ExternalLabels: map[string]string{
					"foo": "bar",
				},
			},
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Logs.Configs))
		require.Equal(t, 1, len(c.Metrics.Configs))
		require.Equal(t, 1, len(c.Logs.Global.ClientConfigs))
		require.Equal(t, c.Logs.Global.ClientConfigs[0].ExternalLabels.LabelSet, model.LabelSet{"foo": "bar"})
		require.Contains(t, c.Metrics.Global.Prometheus.ExternalLabels, labels.Label{Name: "foo", Value: "bar"})
	})

	t.Run("external labels don't override base config", func(t *testing.T) {
		baseConfig := `
server:
    log_level: debug
metrics:
    global:
        external_labels:
            foo: bar
logs:
    global:
        clients:
        - external_labels:
            foo: bar
`
		rc := RemoteConfig{
			BaseConfig: BaseConfigContent(baseConfig),
			Snippets:   allSnippets,
			AgentMetadata: AgentMetadata{
				ExternalLabels: map[string]string{
					"foo": "baz",
				},
			},
		}
		c, err := rc.BuildAgentConfig()
		require.NoError(t, err)
		require.Equal(t, 1, len(c.Logs.Configs))
		require.Equal(t, 1, len(c.Metrics.Configs))
		require.Equal(t, 1, len(c.Logs.Global.ClientConfigs))
		require.Equal(t, c.Logs.Global.ClientConfigs[0].ExternalLabels.LabelSet, model.LabelSet{"foo": "bar"})
		require.Contains(t, c.Metrics.Global.Prometheus.ExternalLabels, labels.Label{Name: "foo", Value: "bar"})
		require.NotContains(t, c.Metrics.Global.Prometheus.ExternalLabels, labels.Label{Name: "foo", Value: "baz"})
	})
}
