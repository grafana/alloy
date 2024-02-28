local agentDashboards = import 'grafana-agent-mixin/dashboards.libsonnet';
local agentDebugging = import 'grafana-agent-mixin/debugging.libsonnet';

local result = agentDashboards + agentDebugging {
  files: {
    [name]: $.grafanaDashboards[name] {
      // Use local timezone for local testing
      timezone: '',
    }
    for name in std.objectFields($.grafanaDashboards)
  },
};

result.files
