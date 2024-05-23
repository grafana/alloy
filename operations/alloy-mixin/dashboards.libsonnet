local alloyClusterDashboards =   
  (import './dashboards/cluster-node.libsonnet') + 
  (import './dashboards/cluster-overview.libsonnet') +
  (import './config.libsonnet');

local otherDashboards =  
  (import './dashboards/resources.libsonnet') +
  (import './dashboards/controller.libsonnet') + 
  (import './dashboards/prometheus.libsonnet') + 
  (import './dashboards/opentelemetry.libsonnet') +
  (import './config.libsonnet');

(import './dashboards/alloy-logs.libsonnet') +
{   
  grafanaDashboards+:     
    if $._config.enableAlloyCluster then 
       alloyClusterDashboards +
       otherDashboards
    else
      otherDashboards
}
