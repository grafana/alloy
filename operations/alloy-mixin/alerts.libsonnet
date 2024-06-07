local clusterAlerts = (import './alerts/clustering.libsonnet');
local controllerAlerts = (import './alerts/controller.libsonnet');
local openTelemetryAlerts = (import './alerts/opentelemetry.libsonnet');

{
  local alloyClusterAlerts = [clusterAlerts.newAlloyClusterAlertsGroup($._config.enableK8sCluster)],

  local otherAlerts = [
    controllerAlerts.newControllerAlertsGroup($._config.enableK8sCluster),
    openTelemetryAlerts.newOpenTelemetryAlertsGroup($._config.enableK8sCluster)
  ],

  prometheusAlerts+:: {
    groups+: 
      if $._config.enableAlloyCluster then
        alloyClusterAlerts + otherAlerts
      else
        otherAlerts
  },
}
