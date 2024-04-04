{
  prometheusAlerts+: {
    groups+: [
      (import './alerts/clustering.libsonnet'),
      (import './alerts/controller.libsonnet'),
      (import './alerts/opentelemetry.libsonnet'),
    ],
  },
}
