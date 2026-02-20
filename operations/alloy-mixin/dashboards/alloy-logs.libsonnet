local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local filename = 'alloy-logs.json';

{
  local filtersTemplateVariable = {
    name: 'filters',
    type: 'adhoc',
    datasource: {
      type: 'prometheus',
      uid: '${datasource}',
    },
  },

  local groupByTemplateVariable = dashboard.newGroupByTemplateVariable(
    query='level,job,instance,namespace,cluster,pod',
    defaultValue='level'
  ),

  // Build the Loki label selector based on config
  local baseLabels = if $._config.enableK8sCluster then
    'cluster=~"$cluster", namespace=~"$namespace", job=~"$job", instance=~"$instance", level=~"$level"'
  else
    'job=~"$job", instance=~"$instance", level=~"$level"',
  
  local logsSelector = 
    if std.length($._config.logsFilterSelector) > 0 then
      '{%s, %s}' % [baseLabels, $._config.logsFilterSelector]
    else
      '{%s}' % baseLabels,

  local lokiTemplateVariables = 
    if $._config.enableK8sCluster then ([
      {
        name: 'cluster',
        label: 'Cluster',
        type: 'query',
        datasource: '${datasource}',
        query: 'label_values(alloy_build_info, cluster)',
        refresh: 2,
        sort: 1,
        multi: true,
        includeAll: true,
        allValue: '.*',
      },
      {
        name: 'namespace',
        label: 'Namespace',
        type: 'query',
        datasource: '${datasource}',
        query: 'label_values(alloy_build_info{cluster=~"$cluster"}, namespace)',
        refresh: 2,
        sort: 1,
        multi: true,
        includeAll: true,
        allValue: '.*',
      },
      {
        name: 'job',
        label: 'Job',
        type: 'query',
        datasource: '${loki_datasource}',
        query: 'label_values({cluster=~"$cluster", namespace=~"$namespace"}, job)',
        refresh: 2,
        sort: 1,
        multi: true,
        includeAll: true,
        allValue: '.*',
      },
      {
        name: 'instance',
        label: 'Instance',
        type: 'query',
        datasource: '${loki_datasource}',
        query: 'label_values({cluster=~"$cluster", namespace=~"$namespace", job=~"$job"}, instance)',
        refresh: 2,
        sort: 1,
        multi: true,
        includeAll: true,
        allValue: '.*',
      },
      {
        name: 'level',
        label: 'Level',
        type: 'query',
        datasource: '${loki_datasource}',
        query: 'label_values({cluster=~"$cluster", namespace=~"$namespace", job=~"$job", instance=~"$instance"}, level)',
        refresh: 2,
        sort: 1,
        multi: true,
        includeAll: true,
        allValue: '.*',
      },
      {
        name: 'regex_search',
        label: 'Regex search',
        type: 'textbox',
        query: '',
        current: {
          selected: false,
          text: '',
          value: '',
        },
        options: [
          {
            selected: true,
            text: '',
            value: '',
          },
        ],
      },
    ] + [filtersTemplateVariable, groupByTemplateVariable])
  else ([
    {
      name: 'job',
      label: 'Job',
      type: 'query',
      datasource: '${loki_datasource}',
      query: 'label_values({}, job)',
      refresh: 2,
      sort: 1,
      multi: true,
      includeAll: true,
      allValue: '.*',
    },
    {
      name: 'instance',
      label: 'Instance',
      type: 'query',
      datasource: '${loki_datasource}',
      query: 'label_values({job=~"$job"}, instance)',
      refresh: 2,
      sort: 1,
      multi: true,
      includeAll: true,
      allValue: '.*',
    },
    {
      name: 'level',
      label: 'Level',
      type: 'query',
      datasource: '${loki_datasource}',
      query: 'label_values({job=~"$job", instance=~"$instance"}, level)',
      refresh: 2,
      sort: 1,
      multi: true,
      includeAll: true,
      allValue: '.*',
    },
    {
      name: 'regex_search',
      label: 'Regex search',
      type: 'textbox',
      query: '',
      current: {
        selected: false,
        text: '',
        value: '',
      },
      options: [
        {
          selected: true,
          text: '',
          value: '',
        },
      ],
    },
  ] + [filtersTemplateVariable, groupByTemplateVariable]),

  grafanaDashboards+::
    if $._config.enableLokiLogs then {
      [filename]:
        dashboard.new(name='Alloy / Logs Overview', tag=$._config.dashboardTag) +
        dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
        dashboard.withUID(std.md5(filename)) +
        dashboard.withTemplateVariablesMixin(lokiTemplateVariables) +
        dashboard.withPanelsMixin([
          // Logs volume panel
          (
            panel.new('Logs volume', 'timeseries') +
            {
              datasource: {
                type: 'loki',
                uid: '${loki_datasource}',
              },
            } +
            panel.withDescription('Logs volume grouped by "level" label.') +
            panel.withPosition({ h: 6, w: 24, x: 0, y: 0 }) +
            panel.withQueries([
              {
                datasource: {
                  type: 'loki',
                  uid: '${loki_datasource}',
                },
                expr: 'sum by (${groupby}) (count_over_time(%s\n|~ "$regex_search"\n\n[$__auto]))\n' % logsSelector,
                legendFormat: '{{${groupby}}}',
              },
            ]) +
            {
              fieldConfig: {
                defaults: {
                  custom: {
                    drawStyle: 'bars',
                    fillOpacity: 50,
                    stacking: {
                      mode: 'normal',
                    },
                  },
                  unit: 'none',
                },
                overrides: [
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: '(E|e)merg|(F|f)atal|(A|a)lert|(C|c)rit.*',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'purple',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: '(E|e)(rr.*|RR.*)',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'red',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: '(W|w)(arn.*|ARN.*|rn|RN)',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'orange',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: '(N|n)(otice|ote)|(I|i)(nf.*|NF.*)',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'green',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: 'dbg.*|DBG.*|(D|d)(EBUG|ebug)',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'blue',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: '(T|t)(race|RACE)',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'light-blue',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                  {
                    matcher: {
                      id: 'byRegexp',
                      options: 'logs',
                    },
                    properties: [
                      {
                        id: 'color',
                        value: {
                          fixedColor: 'text',
                          mode: 'fixed',
                        },
                      },
                    ],
                  },
                ],
              },
              options: {
                tooltip: {
                  mode: 'multi',
                  sort: 'desc',
                },
              },
              transformations: [
                {
                  id: 'renameByRegex',
                  options: {
                    regex: 'Value',
                    renamePattern: 'logs',
                  },
                },
              ],
            }
          ),
          // Logs panel
          (
            panel.new('Logs', 'logs') +
            {
              datasource: {
                type: 'datasource',
                uid: '-- Mixed --',
              },
            } +
            panel.withPosition({ h: 18, w: 24, x: 0, y: 6 }) +
            panel.withQueries([
              {
                datasource: {
                  type: 'loki',
                  uid: '${loki_datasource}',
                },
                expr: '%s \n|~ "$regex_search"\n\n\n' % logsSelector,
              },
            ]) +
            {
              options: {
                dedupStrategy: 'exact',
                enableLogDetails: true,
                prettifyLogMessage: true,
                showTime: false,
                wrapLogMessage: true,
              },
            }
          ),
        ]),
    } else {},
}
