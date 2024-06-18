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
            'sum without (cluster, namespace, job, instance, component_path, component_id) (rate(alloy_component_evaluation_slow_seconds[10m])) > 0'
          else
            'sum without (job, instance, component_path, component_id) (rate(alloy_component_evaluation_slow_seconds[10m])) > 0'
          ,
          'Component evaluations are taking too long.',
          'Component evaluations are taking too long for instance {{ $labels.instance }}, component_path {{ $labels.component_path }}, component_id {{ $labels.component_id }}.',
          '15m',
        ),

        // Unhealthy components detected.
        alert.newRule(
          'UnhealthyComponents',
          if enableK8sCluster then
            'sum without (cluster, namespace, job, instance) (alloy_component_controller_running_components{health_type!="healthy"}) > 0'
          else
            'sum without (job, instance) (alloy_component_controller_running_components{health_type!="healthy"}) > 0'
          ,
          'Unhealthy components detected.',
          'Unhealthy components detected within instance {{ $labels.instance }}',
          '15m',
        ),
      ]
    )
}
