local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local filename = 'agent-flow-controller.json';

{
  [filename]:
    dashboard.new(name='Grafana Agent Flow / Controller') +
    dashboard.withDocsLink(
      url='https://grafana.com/docs/agent/latest/flow/concepts/component_controller/',
      desc='Component controller documentation',
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
      // Running agents
      (
        panel.newSingleStat('Running agents') +
        panel.withUnit('agents') +
        panel.withDescription(|||
          The number of Grafana Agent Flow instances whose metrics are being sent and reported.
        |||) +
        panel.withPosition({ x: 0, y: 0, w: 10, h: 4 }) +
        panel.withQueries([
          panel.newQuery(
            expr='count(agent_component_controller_evaluating{cluster="$cluster", namespace="$namespace"})',
          ),
        ])
      ),

      // Running components
      (
        panel.newSingleStat('Running components') +
        panel.withUnit('components') +
        panel.withDescription(|||
          The number of running components across all running agents.
        |||) +
        panel.withPosition({ x: 0, y: 4, w: 10, h: 4 }) +
        panel.withQueries([
          panel.newQuery(
            expr='sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace"})',
          ),
        ])
      ),

      // Overall component health
      (
        panel.newGraphedSingleStat('Overall component health') {
          fieldConfig: {
            defaults: {
              min: 0,
              max: 1,
              noValue: 'No components',
            },
          },
        } +
        panel.withUnit('percentunit') +
        panel.withDescription(|||
          The percentage of components which are in a healthy state.
        |||) +
        panel.withPosition({ x: 0, y: 8, w: 10, h: 4 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace",health_type="healthy"}) /
              sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace"})
            |||,
          ),
        ])
      ),

      // Components by health
      (
        panel.new(title='Components by health', type='bargauge') {
          options: {
            orientation: 'vertical',
            showUnfilled: true,
          },
          fieldConfig: {
            defaults: {
              min: 0,
              thresholds: {
                mode: 'absolute',
                steps: [{ color: 'green', value: null }],
              },
            },
            overrides: [
              {
                matcher: { id: 'byName', options: 'Unhealthy' },
                properties: [{
                  id: 'thresholds',
                  value: {
                    mode: 'absolute',
                    steps: [
                      { color: 'green', value: null },
                      { color: 'red', value: 1 },
                    ],
                  },
                }],
              },
              {
                matcher: { id: 'byName', options: 'Unknown' },
                properties: [{
                  id: 'thresholds',
                  value: {
                    mode: 'absolute',
                    steps: [
                      { color: 'green', value: null },
                      { color: 'blue', value: 1 },
                    ],
                  },
                }],
              },
              {
                matcher: { id: 'byName', options: 'Exited' },
                properties: [{
                  id: 'thresholds',
                  value: {
                    mode: 'absolute',
                    steps: [
                      { color: 'green', value: null },
                      { color: 'orange', value: 1 },
                    ],
                  },
                }],
              },
            ],
          },
        } +
        panel.withDescription(|||
          Breakdown of components by health across all running agents.

          * Healthy: components have been evaluated completely and are reporting themselves as healthy.
          * Unhealthy: Components either could not be evaluated or are reporting themselves as unhealthy.
          * Unknown: A component has been created but has not yet been started.
          * Exited: A component has exited. It will not return to the running state.

          More information on a component's health state can be retrieved using
          the Grafana Agent Flow UI.

          Note that components may be in a degraded state even if they report
          themselves as healthy. Use component-specific dashboards and alerts
          to observe detailed information about the behavior of a component.
        |||) +
        panel.withPosition({ x: 10, y: 0, w: 14, h: 12 }) +
        panel.withQueries([
          panel.newInstantQuery(
            legendFormat='Healthy',
            expr='sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace", health_type="healthy"}) or vector(0)',
          ),
          panel.newInstantQuery(
            legendFormat='Unhealthy',
            expr='sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace", health_type="unhealthy"}) or vector(0)',
          ),
          panel.newInstantQuery(
            legendFormat='Unknown',
            expr='sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace", health_type="unknown"}) or vector(0)',
          ),
          panel.newInstantQuery(
            legendFormat='Exited',
            expr='sum(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace", health_type="exited"}) or vector(0)',
          ),
        ])
      ),

      // Component evaluation rate
      (
        panel.new(title='Component evaluation rate', type='timeseries') {
          fieldConfig: {
            defaults: {
              custom: {
                drawStyle: 'points',
                pointSize: 3,
              },
            },
          },
        } +
        panel.withUnit('ops') +
        panel.withDescription(|||
          The frequency at which components get updated.
        |||) +
        panel.withPosition({ x: 0, y: 12, w: 8, h: 10 }) +
        panel.withMultiTooltip() +
        panel.withQueries([
          panel.newQuery(
            expr='sum by (instance) (rate(agent_component_evaluation_seconds_count{cluster="$cluster", namespace="$namespace"}[$__rate_interval]))',
          ),
        ])
      ),

      // Component evaluation time
      (
        panel.new(title='Component evaluation time', type='timeseries') +
        panel.withUnit('s') +
        panel.withDescription(|||
          The percentiles for how long it takes to complete component evaluations.

          Component evaluations must complete for components to have the latest
          arguments. The longer the evaluations take, the slower it will be to
          reconcile the state of components.

          If evaluation is taking too long, consider sharding your components to
          deal with smaller amounts of data and reuse data as much as possible.
        |||) +
        panel.withPosition({ x: 8, y: 12, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr='histogram_quantile(0.99, sum by (le) (rate(agent_component_evaluation_seconds_bucket{cluster="$cluster",namespace="$namespace"}[$__rate_interval])))',
            legendFormat='99th percentile',
          ),
          panel.newQuery(
            expr='histogram_quantile(0.50, sum by (le) (rate(agent_component_evaluation_seconds_bucket{cluster="$cluster",namespace="$namespace"}[$__rate_interval])))',
            legendFormat='50th percentile',
          ),
          panel.newQuery(
            expr=|||
              sum(rate(agent_component_evaluation_seconds_sum{cluster="$cluster",namespace="$namespace"}[$__rate_interval])) /
              sum(rate(agent_component_evaluation_seconds_count{cluster="$cluster",namespace="$namespace"}[$__rate_interval]))
            |||,
            legendFormat='Average',
          ),
        ])
      ),

      // Slow components evaluation time %
      (
        panel.new(title='Slow components evaluation times', type='timeseries') +
        panel.withUnit('percentunit') +
        panel.withDescription(|||
          The percentage of time spent evaluating 'slow' components - components that took longer than 1 minute to evaluate.

          Ideally, no component should take more than 1 minute to evaluate. The components displayed in this chart
          may be a sign of a problem with the pipeline.
        |||) +
        panel.withPosition({ x: 16, y: 12, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              sum by (component_id) (rate(agent_component_evaluation_slow_seconds{cluster="$cluster", namespace="$namespace"}[$__rate_interval]))
              / scalar(sum(rate(agent_component_evaluation_seconds_sum{cluster="$cluster", namespace="$namespace"}[$__rate_interval])))
            |||,
            legendFormat='{{component_id}}',
          ),
        ])
      ),

      // Component evaluation histogram
      (
        panel.newHeatmap('Component evaluation histogram') +
        panel.withDescription(|||
          Detailed histogram view of how long component evaluations take.

          The goal is to design your config so that evaluations take as little
          time as possible; under 100ms is a good goal.
        |||) +
        panel.withPosition({ x: 0, y: 22, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr='sum by (le) (increase(agent_component_evaluation_seconds_bucket{cluster="$cluster", namespace="$namespace"}[$__rate_interval]))',
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),

      // Component dependency wait time histogram
      (
        panel.newHeatmap('Component dependency wait histogram') +
        panel.withDescription(|||
          Detailed histogram of how long components wait to be evaluated after their dependency is updated.

          The goal is to design your config so that most of the time components do not
          queue for long; under 10ms is a good goal.
        |||) +
        panel.withPosition({ x: 8, y: 22, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr='sum by (le) (increase(agent_component_dependencies_wait_seconds_bucket{cluster="$cluster", namespace="$namespace"}[$__rate_interval]))',
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),
    ]),
}
