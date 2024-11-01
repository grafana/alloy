local mixin = import '../mixin.libsonnet';

{


  dashboards: {
    [file]: {
      apiVersion: 'grizzly.grafana.com/v1alpha1',
      kind: 'Dashboard',
      metadata: {
        folder: 'general',
        name: std.md5(file),
      },
      spec: mixin.grafanaDashboards[file],
    }
    for file in std.objectFields(mixin.grafanaDashboards)
  },
}
