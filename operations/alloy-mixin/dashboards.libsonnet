(import './dashboards/alloy-logs.libsonnet') +
{
  //declare config here to propagate it to the dashboards.
  local config = {_config:: $._config},

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

  grafanaDashboards+::
    if $._config.enableAlloyCluster then 
       alloyClusterDashboards +
       otherDashboards
    else
      otherDashboards
}
