local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local filename = 'agent-cluster-overview.json';
local cluster_node_filename = 'agent-cluster-node.json';

{
  [filename]:
    dashboard.new(name='Grafana Agent Flow / Cluster Overview') +
    dashboard.withDocsLink(
      url='https://grafana.com/docs/agent/latest/flow/reference/cli/run/#clustered-mode-experimental',
      desc='Clustering documentation',
    ) +
    dashboard.withDashboardsLink() +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin([
      dashboard.newTemplateVariable('cluster', |||
        label_values(agent_component_controller_running_components, cluster)
      |||),
      dashboard.newTemplateVariable('namespace', |||
        label_values(agent_component_controller_running_components{cluster="$cluster"}, namespace)
      |||),
    ]) +
    // TODO(@tpaschalis) Make the annotation optional.
    dashboard.withAnnotations([
      dashboard.newLokiAnnotation('Deployments', '{cluster="$cluster", container="kube-diff-logger"} | json | namespace_extracted="grafana-agent" | name_extracted=~"grafana-agent.*"', 'rgba(0, 211, 255, 1)'),
    ]) +
    dashboard.withPanelsMixin([
      // Nodes
      (
        panel.new('Nodes', 'stat') +
        panel.withPosition({ h: 9, w: 8, x: 0, y: 0 }) +
        panel.withQueries([
          panel.newInstantQuery(
            expr='count(cluster_node_info{cluster="$cluster", namespace="$namespace"})'
          ),
        ])
      ),
      // Node table
      (
        panel.new('Node table', 'table') +
        panel.withDescription(|||
          Nodes info.
        |||) +
        panel.withPosition({ h: 9, w: 16, x: 8, y: 0 }) +
        panel.withQueries([
          panel.newInstantQuery(
            expr='cluster_node_info{cluster="$cluster", namespace="$namespace"}',
            format='table',
          ),
        ]) +
        panel.withTransformations([
          {
            id: 'organize',
            options: {
              excludeByName: {
                Time: true,
                Value: false,
                __name__: true,
                cluster: true,
                namespace: true,
                state: false,
              },
              indexByName: {},
              renameByName: {
                Value: 'Dashboard',
                instance: '',
                state: '',
              },
            },
          },
        ]) +
        panel.withOverrides(
          [
            {
              matcher: {
                id: 'byName',
                options: 'Dashboard',
              },
              properties: [
                {
                  id: 'mappings',
                  value: [
                    {
                      options: {
                        '1': {
                          index: 0,
                          text: 'Link',
                        },
                      },
                      type: 'value',
                    },
                  ],
                },
                {
                  id: 'links',
                  value: [
                    {
                      targetBlank: false,
                      title: 'Detail dashboard for node',
                      url: '/d/%(uid)s/grafana-agent-flow-cluster-node?var-instance=${__data.fields.instance}&var-datasource=${datasource}&var-loki_datasource=${loki_datasource}&var-cluster=${cluster}&var-namespace=${namespace}' % { uid: std.md5(cluster_node_filename) },
                    },
                  ],
                },
              ],
            },
          ],
        )
      ),
      // Convergance state
      (
        panel.new('Convergance state', 'stat') +
        panel.withDescription(|||
          Whether the cluster state has converged.

          It is normal for the cluster state to be diverged briefly as gossip events propagate. It is not normal for the cluster state to be diverged for a long period of time.

          This will show one of the following:

          * Converged: Nodes are aware of all other nodes, with the correct states.
          * Not converged: A subset of nodes aren't aware of their peers, or don't have an updated view of peer states.
        |||) +
        panel.withPosition({ h: 9, w: 8, x: 0, y: 9 }) +
        panel.withQueries([
          panel.newInstantQuery(
            expr=|||
              clamp((
                sum(stddev by (state) (cluster_node_peers{cluster="$cluster", namespace="$namespace"}) != 0) or
                (sum(abs(sum without (state) (cluster_node_peers{cluster="$cluster", namespace="$namespace"})) - scalar(count(cluster_node_info{cluster="$cluster", namespace="$namespace"})) != 0))
                ),
                1, 1
              )
            |||,
            format='time_series'
          ),
        ]) +
        panel.withOptions(
          {
            colorMode: 'background',
            graphMode: 'none',
            justifyMode: 'auto',
            orientation: 'auto',
            reduceOptions: {
              calcs: [
                'lastNotNull',
              ],
              fields: '',
              values: false,
            },
            textMode: 'auto',
          }
        ) +
        panel.withMappings([
          {
            options: {
              '1': {
                color: 'red',
                index: 1,
                text: 'Not converged',
              },
            },
            type: 'value',
          },
          {
            options: {
              match: 'null',
              result: {
                color: 'green',
                index: 0,
                text: 'Converged',
              },
            },
            type: 'special',
          },
        ]) +
        panel.withUnit('suffix:nodes')
      ),
      // Convergance state timeline
      (
        panel.new('Convergance state timeline', 'state-timeline') {
          fieldConfig: {
            defaults: {
              custom: {
                fillOpacity: 80,
                spanNulls: true,
              },
              noValue: 0,
              max: 1,
            },
          },
        } +
        panel.withPosition({ h: 9, w: 16, x: 8, y: 9 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              ceil(clamp((
                sum(stddev by (state) (cluster_node_peers{cluster="$cluster", namespace="$namespace"})) or
                (sum(abs(sum without (state) (cluster_node_peers{cluster="$cluster", namespace="$namespace"})) - scalar(count(cluster_node_info{cluster="$cluster", namespace="$namespace"}))))
                ),
                0, 1
              ))
            |||,
            legendFormat='Converged'
          ),
        ]) +
        panel.withOptions({
          mergeValues: true,
        }) +
        panel.withMappings([
          {
            options: {
              '0': { color: 'green', text: 'Yes' },
            },
            type: 'value',
          },
          {
            options: {
              '1': { color: 'red', text: 'No' },
            },
            type: 'value',
          },
        ])
      ),
    ]),
}
