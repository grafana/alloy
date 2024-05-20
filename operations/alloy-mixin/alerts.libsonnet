local clusterAlerts = (import './alerts/clustering.libsonnet');
local controllerAlerts = (import './alerts/controller.libsonnet');
local openTelemetryAlerts = (import './alerts/opentelemetry.libsonnet');

{
  prometheusAlerts+: {
    groups+: 
    if $._config.enableAlloyCluster then
      [      
        clusterAlerts.newAlloyClusterAlertsGroup($._config.enableK8sCluster),
        controllerAlerts.newControllerAlertsGroup($._config.enableK8sCluster),
        openTelemetryAlerts.newOpenTelemetryAlertsGroup($._config.enableK8sCluster),
      ]
      else         
      [
        controllerAlerts.newControllerAlertsGroup($._config.enableK8sCluster),
        openTelemetryAlerts.newOpenTelemetryAlertsGroup($._config.enableK8sCluster)
      ],
  },
}
