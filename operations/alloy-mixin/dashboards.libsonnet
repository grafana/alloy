(import './dashboards/alloy-logs.libsonnet') +
{
  local alloyClusterDashboards =   
    (import './dashboards/cluster-node.libsonnet') + 
    (import './dashboards/cluster-overview.libsonnet'),
  local otherDashboards =  
    (import './dashboards/resources.libsonnet') +
    (import './dashboards/controller.libsonnet') + 
    (import './dashboards/prometheus.libsonnet') + 
    (import './dashboards/opentelemetry.libsonnet'),

  grafanaDashboards+::
  // Propagate config down to inner dashboards.
  { _config:: $._config } + (
    if $._config.enableAlloyCluster then 
      alloyClusterDashboards + 
      otherDashboards
    else 
      otherDashboards 
  )
}
