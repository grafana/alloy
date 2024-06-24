// alert.jsonnet defines utilities to create alerts.

{
  newGroup(name, rules):: {
    name: name,
    rules: rules,
  },

  newRule(name='', expr='', message='', description='', forT='', severity='warning'):: std.prune({
    alert: name,
    expr: expr,
    annotations: {
      summary: message,
      description: description,
    },
    'for': forT,
    labels: {
      severity: severity,
    },
  }),
}
