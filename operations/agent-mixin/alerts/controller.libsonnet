local alert = import './utils/alert.jsonnet';

alert.newGroup(
  'agent_controller',
  [
    // Component evaluations are taking too long, which can lead to e.g. stale targets.
    alert.newRule(
      'SlowComponentEvaluations',
      'sum by (cluster, namespace, component_id) (rate(agent_component_evaluation_slow_seconds[10m])) > 0',
      'Flow component evaluations are taking too long.',
      '15m',
    ),

    // Unhealthy components detected.
    alert.newRule(
      'UnhealthyComponents',
      'sum by (cluster, namespace) (agent_component_controller_running_components{health_type!="healthy"}) > 0',
      'Unhealthy Flow components detected.',
      '15m',
    ),
  ]
)
