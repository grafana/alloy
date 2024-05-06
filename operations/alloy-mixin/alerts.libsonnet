{
  prometheusAlerts+: {
    groups+: [
      if $._config.enableK8sCluster then 
        (import './alerts/clustering.libsonnet') 
      else 
        {}
      + (import './alerts/controller.libsonnet')
      + (import './alerts/opentelemetry.libsonnet')
    ],
  },
}
