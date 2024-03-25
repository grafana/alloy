local mixin = import '../mixin.libsonnet';

{
  folder: {
    apiVersion: 'grizzly.grafana.com/v1alpha1',
    kind: 'DashboardFolder',
    metadata: {
      name: 'grafana-agent-flow',
    },
    spec: {
      title: mixin.grafanaDashboardFolder,
    },
  },

  dashboards: {
    [file]: {
      apiVersion: 'grizzly.grafana.com/v1alpha1',
      kind: 'Dashboard',
      metadata: {
        folder: $.folder.metadata.name,
        name: std.md5(file),
      },
      spec: mixin.grafanaDashboards[file],
    }
    for file in std.objectFields(mixin.grafanaDashboards)
  },
}
