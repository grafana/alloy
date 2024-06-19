local g = import 'github.com/grafana/grafonnet/gen/grafonnet-v10.0.0/main.libsonnet';
local logsDashboard = import 'github.com/grafana/jsonnet-libs/logs-lib/logs/main.libsonnet';

{

  local labels = if $._config.enableK8sCluster then ['cluster', 'namespace', 'job', 'instance', 'level'] else ['job', 'instance', 'level'],

  grafanaDashboards+:
    if $._config.enableLokiLogs then {
      local alloyLogs =
        logsDashboard.new(
          'Alloy / Logs Overview',
          datasourceName='loki_datasource',
          datasourceRegex='',
          filterSelector=$._config.logsFilterSelector,
          labels=labels,
          formatParser=null,
          showLogsVolume=true
        )
        {
          panels+:
            {
              logs+:
                // Alloy logs already have timestamp
                g.panel.logs.options.withShowTime(false),
            },
          dashboards+:
            {
              logs+: g.dashboard.withLinksMixin($.grafanaDashboards['alloy-resources.json'].links)                     
                     + g.dashboard.withRefresh('10s')
                     + g.dashboard.withTagsMixin($._config.dashboardTag),
            },
        },
      'alloy-logs.json': alloyLogs.dashboards.logs,
    } else {},
}
