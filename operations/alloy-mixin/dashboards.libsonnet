{
  grafanaDashboards+:
    (import './dashboards/controller.libsonnet') +
    (import './dashboards/resources.libsonnet') +
    (import './dashboards/prometheus.libsonnet') +
    (import './dashboards/cluster-node.libsonnet') +
    (import './dashboards/opentelemetry.libsonnet') +
    (import './dashboards/cluster-overview.libsonnet'),
}
