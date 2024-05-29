local alert = import './utils/alert.jsonnet';

{
  newControllerAlertsGroup(enableK8sCluster=true):
    alert.newGroup(
      'alloy_controller',
      [
        // Component evaluations are taking too long, which can lead to e.g. stale targets.
        alert.newRule(
          'SlowComponentEvaluations',
          if enableK8sCluster then
            'sum by (cluster, namespace, job, component_path, component_id) (rate(alloy_component_evaluation_slow_seconds[10m])) > 0'
          else
            'sum by (job, component_path, component_id) (rate(alloy_component_evaluation_slow_seconds[10m])) > 0'
          ,
          'Component evaluations are taking too long.',
          '15m',
        ),

        // Unhealthy components detected.
        alert.newRule(
          'UnhealthyComponents',
          if enableK8sCluster then
            'sum by (cluster, namespace, job) (alloy_component_controller_running_components{health_type!="healthy"}) > 0'
          else
            'sum by (job) (alloy_component_controller_running_components{health_type!="healthy"}) > 0'
          ,
          'Unhealthy components detected.',
          '15m',
        ),
      ]
    )
}
