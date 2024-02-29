package operator

// Supported versions of the Grafana Agent.
var (
	DefaultAgentVersion   = "$AGENT_VERSION"
	DefaultAgentBaseImage = "grafana/agent"
	DefaultAgentImage     = DefaultAgentBaseImage + ":" + DefaultAgentVersion
)

// Defaults for Prometheus Config Reloader.
var (
	DefaultConfigReloaderVersion   = "v0.67.1"
	DefaultConfigReloaderBaseImage = "quay.io/prometheus-operator/prometheus-config-reloader"
	DefaultConfigReloaderImage     = DefaultConfigReloaderBaseImage + ":" + DefaultConfigReloaderVersion
)
