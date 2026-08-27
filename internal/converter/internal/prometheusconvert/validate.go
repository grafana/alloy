package prometheusconvert

import (
	"reflect"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/component"
	prom_config "github.com/prometheus/prometheus/config"
	prom_discover "github.com/prometheus/prometheus/discovery"

	_ "github.com/prometheus/prometheus/discovery/install" // Register Prometheus SDs
)

func validate(promConfig *prom_config.Config) diag.Diagnostics {
	diags := validateGlobalConfig(&promConfig.GlobalConfig)
	diags.AddAll(validateAlertingConfig(&promConfig.AlertingConfig))
	diags.AddAll(validateRuleFilesConfig(promConfig.RuleFiles))
	diags.AddAll(validateScrapeConfigs(promConfig.ScrapeConfigs))
	diags.AddAll(validateStorageConfig(&promConfig.StorageConfig))
	diags.AddAll(validateTracingConfig(&promConfig.TracingConfig))
	diags.AddAll(validateRemoteWriteConfigs(promConfig.RemoteWriteConfigs))
	diags.AddAll(validateRemoteReadConfigs(promConfig.RemoteReadConfigs))

	return diags
}

func validateGlobalConfig(globalConfig *prom_config.GlobalConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddAll(common.ValidateSupported(common.NotEquals, globalConfig.EvaluationInterval, prom_config.DefaultGlobalConfig.EvaluationInterval, "global evaluation_interval", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, globalConfig.QueryLogFile, "", "global query_log_file", ""))

	return diags
}

func validateAlertingConfig(alertingConfig *prom_config.AlertingConfig) diag.Diagnostics {
	hasAlerting := len(alertingConfig.AlertmanagerConfigs) > 0 || len(alertingConfig.AlertRelabelConfigs) > 0
	return common.ValidateSupported(common.Equals, hasAlerting, true, "alerting", "")
}

func validateRuleFilesConfig(ruleFilesConfig []string) diag.Diagnostics {
	return common.ValidateSupported(common.Equals, len(ruleFilesConfig) > 0, true, "rule_files", "")
}

func validateScrapeConfigs(scrapeConfigs []*prom_config.ScrapeConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, scrapeConfig := range scrapeConfigs {
		diags.AddAll(component.ValidatePrometheusScrape(scrapeConfig))
		diags.AddAll(ValidateServiceDiscoveryConfigs(scrapeConfig.ServiceDiscoveryConfigs))
	}
	return diags
}

func ValidateServiceDiscoveryConfigs(serviceDiscoveryConfigs prom_discover.Configs) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, serviceDiscoveryConfig := range serviceDiscoveryConfigs {
		diags.AddAll(component.ValidateServiceDiscoveryConfig(serviceDiscoveryConfig))
	}

	return diags
}

func validateStorageConfig(storageConfig *prom_config.StorageConfig) diag.Diagnostics {
	// Prometheus v0.311+ always injects default TSDBConfig and ExemplarsConfig during
	// unmarshaling even when no storage block is present in the config file. Compare
	// against the known defaults to detect only explicitly-configured storage.
	defaultRetention := prom_config.DefaultTSDBRetentionConfig
	defaultTSDB := &prom_config.TSDBConfig{Retention: &defaultRetention}
	tsdbConfigured := storageConfig.TSDBConfig != nil && !reflect.DeepEqual(storageConfig.TSDBConfig, defaultTSDB)
	exemplarsConfigured := storageConfig.ExemplarsConfig != nil && !reflect.DeepEqual(*storageConfig.ExemplarsConfig, prom_config.DefaultExemplarsConfig)
	hasStorage := tsdbConfigured || exemplarsConfigured
	return common.ValidateSupported(common.Equals, hasStorage, true, "storage", "")
}

func validateTracingConfig(tracingConfig *prom_config.TracingConfig) diag.Diagnostics {
	return common.ValidateSupported(common.NotDeepEquals, *tracingConfig, prom_config.TracingConfig{}, "tracing", "")
}

func validateRemoteWriteConfigs(remoteWriteConfigs []*prom_config.RemoteWriteConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, remoteWriteConfig := range remoteWriteConfigs {
		diags.AddAll(component.ValidateRemoteWriteConfig(remoteWriteConfig))
	}

	return diags
}

func validateRemoteReadConfigs(remoteReadConfigs []*prom_config.RemoteReadConfig) diag.Diagnostics {
	return common.ValidateSupported(common.Equals, len(remoteReadConfigs) > 0, true, "remote_read", "")
}
