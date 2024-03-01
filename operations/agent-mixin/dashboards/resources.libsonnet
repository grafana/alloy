local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local filename = 'agent-flow-resources.json';

local pointsMixin = {
  fieldConfig+: {
    defaults+: {
      custom: {
        drawStyle: 'points',
        pointSize: 3,
      },
    },
  },

};

local stackedPanelMixin = {
  fieldConfig+: {
    defaults+: {
      custom+: {
        fillOpacity: 30,
        gradientMode: 'none',
        stacking: { mode: 'normal' },
      },
    },
  },
};

{
  [filename]:
    dashboard.new(name='Grafana Agent Flow / Resources') +
    dashboard.withDashboardsLink() +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin([
      dashboard.newTemplateVariable('cluster', |||
        label_values(agent_component_controller_running_components, cluster)
      |||),
      dashboard.newTemplateVariable('namespace', |||
        label_values(agent_component_controller_running_components{cluster="$cluster"}, namespace)
      |||),
      dashboard.newMultiTemplateVariable('instance', |||
        label_values(agent_component_controller_running_components{cluster="$cluster", namespace="$namespace"}, instance)
      |||),
    ]) +
    // TODO(@tpaschalis) Make the annotation optional.
    dashboard.withAnnotations([
      dashboard.newLokiAnnotation('Deployments', '{cluster="$cluster", container="kube-diff-logger"} | json | namespace_extracted="grafana-agent" | name_extracted=~"grafana-agent.*"', 'rgba(0, 211, 255, 1)'),
    ]) +
    dashboard.withPanelsMixin([
      // CPU usage
      (
        panel.new(title='CPU usage', type='timeseries') +
        panel.withUnit('percentunit') +
        panel.withDescription(|||
          CPU usage of the Grafana Agent process relative to 1 CPU core.

          For example, 100% means using one entire CPU core.
        |||) +
        panel.withPosition({ x: 0, y: 0, w: 12, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            expr='rate(agent_resources_process_cpu_seconds_total{cluster="$cluster",namespace="$namespace",instance=~"$instance"}[$__rate_interval])',
            legendFormat='{{instance}}'
          ),
        ])
      ),

      // Memory (RSS)
      (
        panel.new(title='Memory (RSS)', type='timeseries') +
        panel.withUnit('decbytes') +
        panel.withDescription(|||
          Resident memory size of the Grafana Agent process.
        |||) +
        panel.withPosition({ x: 12, y: 0, w: 12, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            expr='agent_resources_process_resident_memory_bytes{cluster="$cluster",namespace="$namespace",instance=~"$instance"}',
            legendFormat='{{instance}}'
          ),
        ])
      ),

      // GCs
      (
        panel.new(title='Garbage collections', type='timeseries') +
        pointsMixin +
        panel.withUnit('ops') +
        panel.withDescription(|||
          Rate at which the Grafana Agent process performs garbage collections.
        |||) +
        panel.withPosition({ x: 0, y: 8, w: 8, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            // Lots of programs export go_goroutines so we ignore anything that
            // doesn't also have a Grafana Agent-specific metric (i.e.,
            // agent_build_info).
            expr=|||
              rate(go_gc_duration_seconds_count{cluster="$cluster",namespace="$namespace",instance=~"$instance"}[5m])
              and on(instance)
              agent_build_info{cluster="$cluster",namespace="$namespace",instance=~"$instance"}
            |||,
            legendFormat='{{instance}}'
          ),
        ])
      ),

      // Goroutines
      (
        panel.new(title='Goroutines', type='timeseries') +
        panel.withUnit('none') +
        panel.withDescription(|||
          Number of goroutines which are running in parallel. An infinitely
          growing number of these indicates a goroutine leak.
        |||) +
        panel.withPosition({ x: 8, y: 8, w: 8, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            // Lots of programs export go_goroutines so we ignore anything that
            // doesn't also have a Grafana Agent-specific metric (i.e.,
            // agent_build_info).
            expr=|||
              go_goroutines{cluster="$cluster",namespace="$namespace",instance=~"$instance"}
              and on(instance)
              agent_build_info{cluster="$cluster",namespace="$namespace",instance=~"$instance"}
            |||,
            legendFormat='{{instance}}'
          ),
        ])
      ),

      // Memory (Go heap inuse)
      (
        panel.new(title='Memory (heap inuse)', type='timeseries') +
        panel.withUnit('decbytes') +
        panel.withDescription(|||
          Heap memory currently in use by the Grafana Agent process.
        |||) +
        panel.withPosition({ x: 16, y: 8, w: 8, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            // Lots of programs export go_memstats_heap_inuse_bytes so we ignore
            // anything that doesn't also have a Grafana Agent-specific metric
            // (i.e., agent_build_info).
            expr=|||
              go_memstats_heap_inuse_bytes{cluster="$cluster",namespace="$namespace",instance=~"$instance"}
              and on(instance)
              agent_build_info{cluster="$cluster",namespace="$namespace",instance=~"$instance"}
            |||,
            legendFormat='{{instance}}'
          ),
        ])
      ),

      // Network RX
      (
        panel.new(title='Network receive bandwidth', type='timeseries') +
        stackedPanelMixin +
        panel.withUnit('Bps') +
        panel.withDescription(|||
          Rate of data received across all network interfaces for the machine
          Grafana Agent is running on.

          Data shown here is across all running processes and not exclusive to
          the running Grafana Agent process.
        |||) +
        panel.withPosition({ x: 0, y: 16, w: 12, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              rate(agent_resources_machine_rx_bytes_total{cluster="$cluster",namespace="$namespace",instance=~"$instance"}[$__rate_interval])
            |||,
            legendFormat='{{instance}}'
          ),
        ])
      ),

      // Network RX
      (
        panel.new(title='Network send bandwidth', type='timeseries') +
        stackedPanelMixin +
        panel.withUnit('Bps') +
        panel.withDescription(|||
          Rate of data sent across all network interfaces for the machine
          Grafana Agent is running on.

          Data shown here is across all running processes and not exclusive to
          the running Grafana Agent process.
        |||) +
        panel.withPosition({ x: 12, y: 16, w: 12, h: 8 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              rate(agent_resources_machine_tx_bytes_total{cluster="$cluster",namespace="$namespace",instance=~"$instance"}[$__rate_interval])
            |||,
            legendFormat='{{instance}}'
          ),
        ])
      ),
    ]),
}
