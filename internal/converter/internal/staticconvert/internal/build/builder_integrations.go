package build

import (
	"fmt"
	"strings"

	"github.com/prometheus/common/model"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/static/config"
	agent_exporter "github.com/grafana/alloy/internal/static/integrations/agent"
	"github.com/grafana/alloy/internal/static/integrations/apache_http"
	"github.com/grafana/alloy/internal/static/integrations/azure_exporter"
	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	"github.com/grafana/alloy/internal/static/integrations/cadvisor"
	"github.com/grafana/alloy/internal/static/integrations/cloudwatch_exporter"
	int_config "github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/grafana/alloy/internal/static/integrations/consul_exporter"
	"github.com/grafana/alloy/internal/static/integrations/dnsmasq_exporter"
	"github.com/grafana/alloy/internal/static/integrations/elasticsearch_exporter"
	"github.com/grafana/alloy/internal/static/integrations/gcp_exporter"
	"github.com/grafana/alloy/internal/static/integrations/github_exporter"
	"github.com/grafana/alloy/internal/static/integrations/kafka_exporter"
	"github.com/grafana/alloy/internal/static/integrations/memcached_exporter"
	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	mssql_exporter "github.com/grafana/alloy/internal/static/integrations/mssql"
	"github.com/grafana/alloy/internal/static/integrations/mysqld_exporter"
	"github.com/grafana/alloy/internal/static/integrations/node_exporter"
	"github.com/grafana/alloy/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
	"github.com/grafana/alloy/internal/static/integrations/process_exporter"
	"github.com/grafana/alloy/internal/static/integrations/redis_exporter"
	"github.com/grafana/alloy/internal/static/integrations/snmp_exporter"
	"github.com/grafana/alloy/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/alloy/internal/static/integrations/squid_exporter"
	"github.com/grafana/alloy/internal/static/integrations/statsd_exporter"
	agent_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/agent"
	apache_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/apache_http"
	app_agent_receiver_v2 "github.com/grafana/alloy/internal/static/integrations/v2/app_agent_receiver"
	blackbox_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/blackbox_exporter"
	common_v2 "github.com/grafana/alloy/internal/static/integrations/v2/common"
	eventhandler_v2 "github.com/grafana/alloy/internal/static/integrations/v2/eventhandler"
	metricsutils_v2 "github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	snmp_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/snmp_exporter"
	"github.com/grafana/alloy/internal/static/integrations/windows_exporter"
	"github.com/grafana/alloy/syntax/scanner"
)

func (b *ConfigBuilder) appendIntegrations() {
	switch b.cfg.Integrations.Version {
	case config.IntegrationsVersion1:
		b.appendV1Integrations()
	case config.IntegrationsVersion2:
		b.appendV2Integrations()
	default:
		panic(fmt.Sprintf("unknown integrations version %d", b.cfg.Integrations.Version))
	}
}

func (b *ConfigBuilder) appendV1Integrations() {
	for _, integration := range b.cfg.Integrations.ConfigV1.Integrations {
		if !integration.Common.Enabled {
			continue
		}

		scrapeIntegration := b.cfg.Integrations.ConfigV1.ScrapeIntegrations
		if integration.Common.ScrapeIntegration != nil {
			scrapeIntegration = *integration.Common.ScrapeIntegration
		}

		if !scrapeIntegration {
			b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("The converter does not support handling integrations which are not being scraped: %s.", integration.Name()))
			continue
		}

		var exports discovery.Exports
		switch itg := integration.Config.(type) {
		case *agent_exporter.Config:
			exports = b.appendAgentExporter(itg)
		case *apache_http.Config:
			exports = b.appendApacheExporter(itg)
		case *node_exporter.Config:
			exports = b.appendNodeExporter(itg, nil)
		case *blackbox_exporter.Config:
			exports = b.appendBlackboxExporter(itg)
		case *cloudwatch_exporter.Config:
			exports = b.appendCloudwatchExporter(itg, nil)
		case *consul_exporter.Config:
			exports = b.appendConsulExporter(itg, nil)
		case *dnsmasq_exporter.Config:
			exports = b.appendDnsmasqExporter(itg, nil)
		case *elasticsearch_exporter.Config:
			exports = b.appendElasticsearchExporter(itg, nil)
		case *gcp_exporter.Config:
			exports = b.appendGcpExporter(itg, nil)
		case *github_exporter.Config:
			exports = b.appendGithubExporter(itg, nil)
		case *kafka_exporter.Config:
			exports = b.appendKafkaExporter(itg, nil)
		case *memcached_exporter.Config:
			exports = b.appendMemcachedExporter(itg, nil)
		case *mongodb_exporter.Config:
			exports = b.appendMongodbExporter(itg, nil)
		case *mssql_exporter.Config:
			exports = b.appendMssqlExporter(itg, nil)
		case *mysqld_exporter.Config:
			exports = b.appendMysqldExporter(itg, nil)
		case *oracledb_exporter.Config:
			exports = b.appendOracledbExporter(itg, nil)
		case *postgres_exporter.Config:
			exports = b.appendPostgresExporter(itg, nil)
		case *process_exporter.Config:
			exports = b.appendProcessExporter(itg, nil)
		case *redis_exporter.Config:
			exports = b.appendRedisExporter(itg, nil)
		case *snmp_exporter.Config:
			exports = b.appendSnmpExporter(itg)
		case *snowflake_exporter.Config:
			exports = b.appendSnowflakeExporter(itg, nil)
		case *squid_exporter.Config:
			exports = b.appendSquidExporter(itg, nil)
		case *statsd_exporter.Config:
			exports = b.appendStatsdExporter(itg, nil)
		case *windows_exporter.Config:
			exports = b.appendWindowsExporter(itg, nil)
		case *azure_exporter.Config:
			exports = b.appendAzureExporter(itg, nil)
		case *cadvisor.Config:
			exports = b.appendCadvisorExporter(itg, nil)
		}

		if len(exports.Targets) > 0 {
			b.appendExporter(&integration.Common, integration.Name(), exports.Targets)
		}
	}
}

func (b *ConfigBuilder) appendExporter(commonConfig *int_config.Common, name string, extraTargets []discovery.Target) {
	var relabelConfigs []*relabel.Config
	if commonConfig.InstanceKey != nil {
		defaultConfig := relabel.DefaultRelabelConfig
		relabelConfig := &defaultConfig
		relabelConfig.TargetLabel = "instance"
		relabelConfig.Replacement = *commonConfig.InstanceKey

		relabelConfigs = append(relabelConfigs, relabelConfig)
	}

	if relabelConfig := b.getJobRelabelConfig(name, commonConfig.RelabelConfigs); relabelConfig != nil {
		relabelConfigs = append(relabelConfigs, b.getJobRelabelConfig(name, commonConfig.RelabelConfigs))
	}

	scrapeConfig := prom_config.DefaultScrapeConfig
	scrapeConfig.JobName = b.formatJobName(name, nil)
	scrapeConfig.RelabelConfigs = append(commonConfig.RelabelConfigs, relabelConfigs...)
	scrapeConfig.MetricRelabelConfigs = commonConfig.MetricRelabelConfigs
	scrapeConfig.HTTPClientConfig.TLSConfig = b.cfg.Integrations.ConfigV1.TLSConfig

	scrapeConfig.ScrapeInterval = model.Duration(commonConfig.ScrapeInterval)
	if commonConfig.ScrapeInterval == 0 {
		scrapeConfig.ScrapeInterval = b.cfg.Integrations.ConfigV1.PrometheusGlobalConfig.ScrapeInterval
	}

	scrapeConfig.ScrapeTimeout = model.Duration(commonConfig.ScrapeTimeout)
	if commonConfig.ScrapeTimeout == 0 {
		scrapeConfig.ScrapeTimeout = b.cfg.Integrations.ConfigV1.PrometheusGlobalConfig.ScrapeTimeout
	}

	// NOTE: We use the default value, since Agent static mode doesn't support setting this.
	scrapeConfig.ScrapeProtocols = prom_config.DefaultScrapeProtocols
	scrapeConfig.MetricNameValidationScheme = prom_config.UTF8ValidationConfig

	scrapeConfigs := []*prom_config.ScrapeConfig{&scrapeConfig}

	promConfig := &prom_config.Config{
		GlobalConfig:       b.cfg.Integrations.ConfigV1.PrometheusGlobalConfig,
		ScrapeConfigs:      scrapeConfigs,
		RemoteWriteConfigs: b.cfg.Integrations.ConfigV1.PrometheusRemoteWrite,
	}

	if len(b.cfg.Integrations.ConfigV1.PrometheusRemoteWrite) == 0 {
		b.diags.Add(diag.SeverityLevelCritical, "The converter does not support handling integrations which are not connected to a remote_write.")
	}

	jobNameToCompLabelsFunc := func(jobName string) string {
		return b.jobNameToCompLabel(jobName)
	}

	b.diags.AddAll(prometheusconvert.AppendAllNested(b.f, promConfig, jobNameToCompLabelsFunc, extraTargets, b.globalCtx.IntegrationsRemoteWriteExports))
	b.globalCtx.InitializeIntegrationsRemoteWriteExports()
}

func (b *ConfigBuilder) appendV2Integrations() {
	for _, integration := range b.cfg.Integrations.ConfigV2.Configs {
		var exports discovery.Exports
		var commonConfig common_v2.MetricsConfig

		switch itg := integration.(type) {
		case *agent_exporter_v2.Config:
			exports = b.appendAgentExporterV2(itg)
			commonConfig = itg.Common
		case *apache_exporter_v2.Config:
			exports = b.appendApacheExporterV2(itg)
			commonConfig = itg.Common
		case *app_agent_receiver_v2.Config:
			b.appendAppAgentReceiverV2(itg)
			commonConfig = itg.Common
		case *blackbox_exporter_v2.Config:
			exports = b.appendBlackboxExporterV2(itg)
			commonConfig = itg.Common
		case *eventhandler_v2.Config:
			b.appendEventHandlerV2(itg)
		case *snmp_exporter_v2.Config:
			exports = b.appendSnmpExporterV2(itg)
			commonConfig = itg.Common
		case *metricsutils_v2.ConfigShim:
			commonConfig = itg.Common
			switch v1_itg := itg.Orig.(type) {
			case *azure_exporter.Config:
				exports = b.appendAzureExporter(v1_itg, itg.Common.InstanceKey)
			case *cadvisor.Config:
				exports = b.appendCadvisorExporter(v1_itg, itg.Common.InstanceKey)
			case *cloudwatch_exporter.Config:
				exports = b.appendCloudwatchExporter(v1_itg, itg.Common.InstanceKey)
			case *consul_exporter.Config:
				exports = b.appendConsulExporter(v1_itg, itg.Common.InstanceKey)
			case *dnsmasq_exporter.Config:
				exports = b.appendDnsmasqExporter(v1_itg, itg.Common.InstanceKey)
			case *elasticsearch_exporter.Config:
				exports = b.appendElasticsearchExporter(v1_itg, itg.Common.InstanceKey)
			case *gcp_exporter.Config:
				exports = b.appendGcpExporter(v1_itg, itg.Common.InstanceKey)
			case *github_exporter.Config:
				exports = b.appendGithubExporter(v1_itg, itg.Common.InstanceKey)
			case *kafka_exporter.Config:
				exports = b.appendKafkaExporter(v1_itg, itg.Common.InstanceKey)
			case *memcached_exporter.Config:
				exports = b.appendMemcachedExporter(v1_itg, itg.Common.InstanceKey)
			case *mongodb_exporter.Config:
				exports = b.appendMongodbExporter(v1_itg, itg.Common.InstanceKey)
			case *mssql_exporter.Config:
				exports = b.appendMssqlExporter(v1_itg, itg.Common.InstanceKey)
			case *mysqld_exporter.Config:
				exports = b.appendMysqldExporter(v1_itg, itg.Common.InstanceKey)
			case *node_exporter.Config:
				exports = b.appendNodeExporter(v1_itg, itg.Common.InstanceKey)
			case *oracledb_exporter.Config:
				exports = b.appendOracledbExporter(v1_itg, itg.Common.InstanceKey)
			case *postgres_exporter.Config:
				exports = b.appendPostgresExporter(v1_itg, itg.Common.InstanceKey)
			case *process_exporter.Config:
				exports = b.appendProcessExporter(v1_itg, itg.Common.InstanceKey)
			case *redis_exporter.Config:
				exports = b.appendRedisExporter(v1_itg, itg.Common.InstanceKey)
			case *snowflake_exporter.Config:
				exports = b.appendSnowflakeExporter(v1_itg, itg.Common.InstanceKey)
			case *squid_exporter.Config:
				exports = b.appendSquidExporter(v1_itg, itg.Common.InstanceKey)
			case *statsd_exporter.Config:
				exports = b.appendStatsdExporter(v1_itg, itg.Common.InstanceKey)
			case *windows_exporter.Config:
				exports = b.appendWindowsExporter(v1_itg, itg.Common.InstanceKey)
			}
		}

		if len(exports.Targets) > 0 {
			b.appendExporterV2(&commonConfig, integration.Name(), exports.Targets)
		}
	}
}

func (b *ConfigBuilder) appendExporterV2(commonConfig *common_v2.MetricsConfig, name string, extraTargets []discovery.Target) {
	var relabelConfigs []*relabel.Config

	for _, extraLabel := range commonConfig.ExtraLabels {
		defaultConfig := relabel.DefaultRelabelConfig
		relabelConfig := &defaultConfig
		relabelConfig.SourceLabels = []model.LabelName{"__address__"}
		relabelConfig.TargetLabel = extraLabel.Name
		relabelConfig.Replacement = extraLabel.Value

		relabelConfigs = append(relabelConfigs, relabelConfig)
	}

	if commonConfig.InstanceKey != nil {
		defaultConfig := relabel.DefaultRelabelConfig
		relabelConfig := &defaultConfig
		relabelConfig.TargetLabel = "instance"
		relabelConfig.Replacement = *commonConfig.InstanceKey

		relabelConfigs = append(relabelConfigs, relabelConfig)
	}

	if relabelConfig := b.getJobRelabelConfig(name, commonConfig.Autoscrape.RelabelConfigs); relabelConfig != nil {
		relabelConfigs = append(relabelConfigs, relabelConfig)
	}

	commonConfig.ApplyDefaults(b.cfg.Integrations.ConfigV2.Metrics.Autoscrape)
	scrapeConfig := prom_config.DefaultScrapeConfig
	scrapeConfig.JobName = b.formatJobName(name, commonConfig.InstanceKey)
	scrapeConfig.RelabelConfigs = append(commonConfig.Autoscrape.RelabelConfigs, relabelConfigs...)
	scrapeConfig.MetricRelabelConfigs = commonConfig.Autoscrape.MetricRelabelConfigs
	scrapeConfig.ScrapeInterval = commonConfig.Autoscrape.ScrapeInterval
	scrapeConfig.ScrapeTimeout = commonConfig.Autoscrape.ScrapeTimeout
	// NOTE: We use the default value, since Agent static mode doesn't support setting this.
	scrapeConfig.ScrapeProtocols = prom_config.DefaultScrapeProtocols
	scrapeConfig.MetricNameValidationScheme = prom_config.UTF8ValidationConfig

	scrapeConfigs := []*prom_config.ScrapeConfig{&scrapeConfig}

	var remoteWriteExports *remotewrite.Exports
	for _, metrics := range b.cfg.Metrics.Configs {
		if metrics.Name == commonConfig.Autoscrape.MetricsInstance {
			// This must match the name of the existing remote write config in the metrics config:
			label, err := scanner.SanitizeIdentifier("metrics_" + metrics.Name)
			if err != nil {
				b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to sanitize job name: %s", err))
			}

			remoteWriteExports = &remotewrite.Exports{
				Receiver: common.ConvertAppendable{Expr: "prometheus.remote_write." + label + ".receiver"},
			}
			break
		}
	}

	if remoteWriteExports == nil {
		b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("integration %s is looking for an undefined metrics config: %s", name, commonConfig.Autoscrape.MetricsInstance))
	}

	promConfig := &prom_config.Config{
		GlobalConfig:  b.cfg.Metrics.Global.Prometheus,
		ScrapeConfigs: scrapeConfigs,
	}

	jobNameToCompLabelsFunc := func(jobName string) string {
		return b.jobNameToCompLabel(jobName)
	}

	// Need to pass in the remote write reference from the metrics config here:
	b.diags.AddAll(prometheusconvert.AppendAllNested(b.f, promConfig, jobNameToCompLabelsFunc, extraTargets, remoteWriteExports))
}

func (b *ConfigBuilder) jobNameToCompLabel(jobName string) string {
	labelSuffix := strings.TrimPrefix(jobName, "integrations/")
	if labelSuffix == "" {
		return b.globalCtx.IntegrationsLabelPrefix
	}

	return fmt.Sprintf("%s_%s", b.globalCtx.IntegrationsLabelPrefix, labelSuffix)
}

func (b *ConfigBuilder) appendExporterBlock(args component.Arguments, configName string, instanceKey *string, exporterName string) discovery.Exports {
	compLabel, err := scanner.SanitizeIdentifier(b.formatJobName(configName, instanceKey))
	if err != nil {
		b.diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to sanitize job name: %s", err))
	}

	b.f.Body().AppendBlock(common.NewBlockWithOverride(
		[]string{"prometheus", "exporter", exporterName},
		compLabel,
		args,
	))

	return common.NewDiscoveryExports(fmt.Sprintf("prometheus.exporter.%s.%s.targets", exporterName, compLabel))
}

func (b *ConfigBuilder) getJobRelabelConfig(name string, relabelConfigs []*relabel.Config) *relabel.Config {
	// Don't add a job relabel if that label is already targeted
	for _, relabelConfig := range relabelConfigs {
		if relabelConfig.TargetLabel == "job" {
			return nil
		}
	}

	defaultConfig := relabel.DefaultRelabelConfig
	relabelConfig := &defaultConfig
	relabelConfig.TargetLabel = "job"
	relabelConfig.Replacement = "integrations/" + name
	return relabelConfig
}

func (b *ConfigBuilder) formatJobName(name string, instanceKey *string) string {
	jobName := b.globalCtx.IntegrationsLabelPrefix
	if instanceKey != nil {
		jobName = fmt.Sprintf("%s/%s", jobName, *instanceKey)
	} else {
		jobName = fmt.Sprintf("%s/%s", jobName, name)
	}

	return jobName
}
