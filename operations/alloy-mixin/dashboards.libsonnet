(import './dashboards/alloy-logs.libsonnet') +
{
  local alloyClusterDashboards =   
    (import './dashboards/cluster-node.libsonnet') + 
    (import './dashboards/cluster-overview.libsonnet') +
    config,

  local otherDashboards =  
    (import './dashboards/resources.libsonnet') +
    (import './dashboards/controller.libsonnet') + 
    (import './dashboards/prometheus.libsonnet') + 
    (import './dashboards/opentelemetry.libsonnet') +
    config,

  local config = {_config:: $._config},

  grafanaDashboards+::
    if $._config.enableAlloyCluster then 
       alloyClusterDashboards +
       otherDashboards
    else
      otherDashboards
}
