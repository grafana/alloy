local mixin = import '../mixin.libsonnet';

{
  prometheus_rules: std.map(
    function(group)
      {
        apiVersion: 'grizzly.grafana.com/v1alpha1',
        kind: 'PrometheusRuleGroup',
        metadata: {
          namespace: 'alloy',
          name: group.name,
        },
        spec: group,
      },
    mixin.prometheusAlerts.groups
  ),
}
