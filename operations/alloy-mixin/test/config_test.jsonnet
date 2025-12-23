local mixin = import '../mixin.libsonnet';

local check(condition, msg) =
  if !condition then
    error '\n\n========================================\n❌ TEST FAILED ❌\n========================================\n\n%s\n\n========================================\n' % msg
  else
    'PASS: %s' % msg;

// List of dashboards to test (without .json extension)
local allDashboards = [
  'alloy-logs',
  'alloy-resources',
  'alloy-controller',
  'alloy-prometheus-remote-write',
  'alloy-opentelemetry',
  'alloy-loki',
];

local clusterDashboards = [
  'alloy-cluster-node',
  'alloy-cluster-overview',
];

// Helper to test if dashboard has cluster/namespace variables
local hasK8sVars(dashboard) =
  local varNames = [v.name for v in dashboard.templating.list];
  std.member(varNames, 'cluster') && std.member(varNames, 'namespace');

// Helper to test if dashboard lacks cluster/namespace variables
local lacksK8sVars(dashboard) =
  local varNames = [v.name for v in dashboard.templating.list];
  !std.member(varNames, 'cluster') && !std.member(varNames, 'namespace');

local noClusterMixin = mixin { _config+:: { enableAlloyCluster: false } };
local clusterMixin = mixin { _config+:: { enableAlloyCluster: true } };
local noK8sMixin = mixin { _config+:: { enableK8sCluster: false } };
local k8sMixin = mixin { _config+:: { enableK8sCluster: true } };
local filterMixin = mixin { _config+:: { logsFilterSelector: 'service_name="alloy"' } };

local tests =
  // Test: enableLokiLogs feature flag
  [
    check(
      !std.objectHas(
        (mixin { _config+:: { enableLokiLogs: false } }).grafanaDashboards,
        'alloy-logs.json'
      ),
      'logs dashboard should not exist when enableLokiLogs=false'
    ),
    check(
      std.objectHas(
        (mixin { _config+:: { enableLokiLogs: true } }).grafanaDashboards,
        'alloy-logs.json'
      ),
      'logs dashboard should exist when enableLokiLogs=true'
    ),
  ] +
  // Test: enableAlloyCluster=false excludes cluster dashboards
  [
    check(
      !std.objectHas(noClusterMixin.grafanaDashboards, name + '.json'),
      '%s should not exist when enableAlloyCluster=false' % name
    )
    for name in clusterDashboards
  ] +
  // Test: enableAlloyCluster=true includes cluster dashboards
  [
    check(
      std.objectHas(clusterMixin.grafanaDashboards, name + '.json'),
      '%s should exist when enableAlloyCluster=true' % name
    )
    for name in clusterDashboards
  ] +
  // Test: enableK8sCluster=false removes cluster/namespace variables from all dashboards
  [
    check(
      lacksK8sVars(noK8sMixin.grafanaDashboards[name + '.json']),
      '%s should not have cluster/namespace vars when enableK8sCluster=false' % name
    )
    for name in allDashboards
  ] +
  // Test: enableK8sCluster=true includes cluster/namespace variables in all dashboards
  [
    check(
      hasK8sVars(k8sMixin.grafanaDashboards[name + '.json']),
      '%s should have cluster/namespace vars when enableK8sCluster=true' % name
    )
    for name in allDashboards
  ] +
  // Test: logsFilterSelector is added to logs queries
  [
    check(
      std.length(std.findSubstr('service_name="alloy"', filterMixin.grafanaDashboards['alloy-logs.json'].panels[0].targets[0].expr)) > 0,
      'logsFilterSelector should be included in alloy-logs query'
    ),
  ];

// Materialize the tests which will evaluate them and we get the results in a list.
{
  result: 'ALL TESTS PASSED',
  total: std.length(tests),
  tests: ['Test %d: %s' % [i, tests[i]] for i in std.range(0, std.length(tests) - 1)],
}

