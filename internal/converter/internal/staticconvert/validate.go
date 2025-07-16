package staticconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/static/config"
	v1 "github.com/grafana/alloy/internal/static/integrations"
	agent_exporter "github.com/grafana/alloy/internal/static/integrations/agent"
	"github.com/grafana/alloy/internal/static/integrations/apache_http"
	"github.com/grafana/alloy/internal/static/integrations/azure_exporter"
	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	"github.com/grafana/alloy/internal/static/integrations/cadvisor"
	"github.com/grafana/alloy/internal/static/integrations/cloudwatch_exporter"
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
	v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	agent_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/agent"
	apache_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/apache_http"
	app_agent_receiver_v2 "github.com/grafana/alloy/internal/static/integrations/v2/app_agent_receiver"
	blackbox_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/blackbox_exporter"
	eventhandler_v2 "github.com/grafana/alloy/internal/static/integrations/v2/eventhandler"
	metricsutils_v2 "github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	snmp_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/snmp_exporter"
	vmware_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/vmware_exporter"
	"github.com/grafana/alloy/internal/static/integrations/windows_exporter"
	"github.com/grafana/alloy/internal/static/logs"
	"github.com/grafana/alloy/internal/static/metrics"
	"github.com/grafana/alloy/internal/static/server"
	"github.com/grafana/alloy/internal/static/traces"

	_ "github.com/prometheus/prometheus/discovery/install" // Register Prometheus SDs
)

func validate(staticConfig *config.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	_, grpcListenPort, _ := staticConfig.ServerFlags.GRPC.ListenHostPort()

	diags.AddAll(validateCommandLine())
	diags.AddAll(validateServer(staticConfig.Server))
	diags.AddAll(validateMetrics(staticConfig.Metrics, grpcListenPort))
	diags.AddAll(validateIntegrations(staticConfig.Integrations))
	diags.AddAll(validateTraces(staticConfig.Traces))
	diags.AddAll(validateLogs(staticConfig.Logs))
	diags.AddAll(validateAgentManagement(staticConfig.AgentManagement))

	return diags
}

func validateCommandLine() diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Add(diag.SeverityLevelWarn, "Please review your agent command line flags and ensure they are set in your Alloy config file where necessary.")

	return diags
}

func validateServer(serverConfig *server.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	defaultServerConfig := server.DefaultConfig()
	diags.AddAll(common.ValidateSupported(common.NotDeepEquals, serverConfig.GRPC, defaultServerConfig.GRPC, "grpc_tls_config server", "Alloy does not have a gRPC server to configure."))
	diags.AddAll(common.ValidateSupported(common.NotEquals, serverConfig.HTTP.TLSConfig.PreferServerCipherSuites, defaultServerConfig.HTTP.TLSConfig.PreferServerCipherSuites, "prefer_server_cipher_suites server", ""))

	return diags
}

// validateMetrics validates the metrics config for anything not already
// covered by appendStaticPrometheus.
func validateMetrics(metricsConfig metrics.Config, grpcListenPort int) diag.Diagnostics {
	var diags diag.Diagnostics

	defaultMetrics := config.DefaultConfig().Metrics
	defaultMetrics.ServiceConfig.Lifecycler.ListenPort = grpcListenPort
	diags.AddAll(common.ValidateSupported(common.NotDeepEquals, metricsConfig.WALCleanupAge, defaultMetrics.WALCleanupAge, "wal_cleanup_age metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, metricsConfig.WALCleanupPeriod, defaultMetrics.WALCleanupPeriod, "wal_cleanup_period metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotYAMLMarshallEquals, metricsConfig.ServiceConfig, defaultMetrics.ServiceConfig, "scraping_service metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotYAMLMarshallEquals, metricsConfig.ServiceClientConfig, defaultMetrics.ServiceClientConfig, "scraping_service_client metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, metricsConfig.InstanceRestartBackoff, defaultMetrics.InstanceRestartBackoff, "instance_restart_backoff metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, metricsConfig.InstanceMode, defaultMetrics.InstanceMode, "instance_mode metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, metricsConfig.DisableKeepAlives, defaultMetrics.DisableKeepAlives, "http_disable_keepalives metrics", ""))
	diags.AddAll(common.ValidateSupported(common.NotEquals, metricsConfig.IdleConnTimeout, defaultMetrics.IdleConnTimeout, "http_idle_conn_timeout metrics", ""))

	if metricsConfig.WALDir != defaultMetrics.WALDir {
		diags.Add(diag.SeverityLevelWarn, "The converter does not support converting the provided metrics wal_directory config: Use the run command flag --storage.path instead.")
	}

	return diags
}

func validateIntegrations(integrationsConfig config.VersionedIntegrations) diag.Diagnostics {
	switch integrationsConfig.Version {
	case config.IntegrationsVersion1:
		return validateIntegrationsV1(integrationsConfig.ConfigV1)
	case config.IntegrationsVersion2:
		return validateIntegrationsV2(integrationsConfig.ConfigV2)
	default:
		panic(fmt.Sprintf("unknown integrations version %d", integrationsConfig.Version))
	}
}

func validateIntegrationsV1(integrationsConfig *v1.ManagerConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, integration := range integrationsConfig.Integrations {
		if !integration.Common.Enabled {
			diags.Add(diag.SeverityLevelWarn, fmt.Sprintf("disabled integrations do nothing and are not included in the output: %s.", integration.Name()))
			continue
		}

		switch itg := integration.Config.(type) {
		case *agent_exporter.Config:
		case *apache_http.Config:
		case *node_exporter.Config:
		case *blackbox_exporter.Config:
		case *cloudwatch_exporter.Config:
		case *consul_exporter.Config:
		case *dnsmasq_exporter.Config:
		case *elasticsearch_exporter.Config:
		case *gcp_exporter.Config:
		case *github_exporter.Config:
		case *kafka_exporter.Config:
		case *memcached_exporter.Config:
		case *mongodb_exporter.Config:
		case *mssql_exporter.Config:
		case *mysqld_exporter.Config:
		case *oracledb_exporter.Config:
		case *postgres_exporter.Config:
		case *process_exporter.Config:
		case *redis_exporter.Config:
		case *snmp_exporter.Config:
		case *snowflake_exporter.Config:
		case *squid_exporter.Config:
		case *statsd_exporter.Config:
		case *windows_exporter.Config:
		case *azure_exporter.Config:
		case *cadvisor.Config:
		default:
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s integration.", itg.Name()))
		}
	}

	return diags
}

func validateIntegrationsV2(integrationsConfig *v2.SubsystemOptions) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, integration := range integrationsConfig.Configs {
		switch itg := integration.(type) {
		case *agent_exporter_v2.Config:
		case *apache_exporter_v2.Config:
		case *app_agent_receiver_v2.Config:
			diags.AddAll(common.ValidateSupported(common.NotEquals, itg.TracesInstance, "", "app_agent_receiver traces_instance", ""))
		case *blackbox_exporter_v2.Config:
		case *eventhandler_v2.Config:
		case *snmp_exporter_v2.Config:
		case *vmware_exporter_v2.Config:
			diags.AddWithDetail(
				diag.SeverityLevelError,
				"Support for the vsphere integration has been removed in Alloy v1.0, and conversion will not be performed.",
				"To achieve similar functionality, consider creating an otelcol.receiver.vcenter component and converting generated metrics to a Prometheus pipeline using otelcol.exporter.prometheus.",
			)
		case *metricsutils_v2.ConfigShim:
			switch v1_itg := itg.Orig.(type) {
			case *azure_exporter.Config:
			case *cadvisor.Config:
			case *cloudwatch_exporter.Config:
			case *consul_exporter.Config:
			case *dnsmasq_exporter.Config:
			case *elasticsearch_exporter.Config:
			case *gcp_exporter.Config:
			case *github_exporter.Config:
			case *kafka_exporter.Config:
			case *memcached_exporter.Config:
			case *mongodb_exporter.Config:
			case *mssql_exporter.Config:
			case *mysqld_exporter.Config:
			case *node_exporter.Config:
			case *oracledb_exporter.Config:
			case *postgres_exporter.Config:
			case *process_exporter.Config:
			case *redis_exporter.Config:
			case *snowflake_exporter.Config:
			case *squid_exporter.Config:
			case *statsd_exporter.Config:
			case *windows_exporter.Config:
			default:
				diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s integration.", v1_itg.Name()))
			}
		default:
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided %s integration.", itg.Name()))
		}
	}

	return diags
}

func validateTraces(_ traces.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

// validateLogs validates the logs config for anything not already covered
// by appendStaticPromtail.
func validateLogs(_ *logs.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

func validateAgentManagement(agentManagementConfig config.AgentManagementConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddAll(common.ValidateSupported(common.NotDeepEquals, agentManagementConfig, config.AgentManagementConfig{}, "agent_management", ""))

	return diags
}
