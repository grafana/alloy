package v1alpha1

import (
	"github.com/grafana/agent/internal/static/operator/assets"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// +genclient

// Deployment is a set of discovered resources relative to a GrafanaAgent. The
// tree of resources contained in a Deployment form the resource hierarchy used
// for reconciling a GrafanaAgent.
type Deployment struct {
	// Root resource in the deployment.
	Agent *GrafanaAgent
	// Metrics resources discovered by Agent.
	Metrics []MetricsDeployment
	// Logs resources discovered by Agent.
	Logs []LogsDeployment
	// Integrations resources discovered by Agent.
	Integrations []IntegrationsDeployment
	// The full list of Secrets referenced by resources in the Deployment.
	Secrets assets.SecretStore
}

// +genclient

// MetricsDeployment is a set of discovered resources relative to a
// MetricsInstance.
type MetricsDeployment struct {
	Instance        *MetricsInstance
	ServiceMonitors []*promv1.ServiceMonitor
	PodMonitors     []*promv1.PodMonitor
	Probes          []*promv1.Probe
}

// +genclient

// LogsDeployment is a set of discovered resources relative to a LogsInstance.
type LogsDeployment struct {
	Instance *LogsInstance
	PodLogs  []*PodLogs
}

// +genclient

// IntegrationsDeployment is a set of discovered resources relative to an
// IntegrationsDeployment.
type IntegrationsDeployment struct {
	Instance *Integration

	// NOTE(rfratto): Integration doesn't have any children resources, but we
	// define a *Deployment type for consistency with Metrics and Logs.
}
