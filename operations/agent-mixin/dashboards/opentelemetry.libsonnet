local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local filename = 'agent-flow-opentelemetry.json';

local stackedPanelMixin = {
  fieldConfig+: {
    defaults+: {
      custom+: {
        fillOpacity: 20,
        gradientMode: 'hue',
        stacking: { mode: 'normal' },
      },
    },
  },
};

{
  [filename]:
    dashboard.new(name='Grafana Agent Flow / OpenTelemetry') +
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
    dashboard.withPanelsMixin([
      // "Receivers for traces" row
      (
        panel.new('Receivers for traces [otelcol.receiver]', 'row') +
        panel.withPosition({ h: 1, w: 24, x: 0, y: 0 })
      ),
      (
        panel.new(title='Accepted spans', type='timeseries') +
        panel.withDescription(|||
         Number of spans successfully pushed into the pipeline.
       |||) +
        panel.withPosition({ x: 0, y: 0, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              rate(receiver_accepted_spans_ratio_total{cluster="$cluster", namespace="$namespace", instance=~"$instance"}[$__rate_interval])
            |||,
            //TODO: How will the dashboard look if there is more than one receiver component? The legend is not unique enough?
            legendFormat='{{ pod }} / {{ transport }}',
          ),
        ])
      ),
      (
        panel.new(title='Refused spans', type='timeseries') +
        stackedPanelMixin +
        panel.withDescription(|||
          Number of spans that could not be pushed into the pipeline.
        |||) +
        panel.withPosition({ x: 8, y: 0, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
             rate(receiver_refused_spans_ratio_total{cluster="$cluster", namespace="$namespace", instance=~"$instance"}[$__rate_interval])
            |||,
            legendFormat='{{ pod }} / {{ transport }}',
          ),
        ])
      ),
      (
        panel.newHeatmap('RPC server duration (traces)') +
        panel.withUnit('milliseconds') +
        panel.withDescription(|||
          The duration of inbound RPCs.
        |||) +
        panel.withPosition({ x: 16, y: 0, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr='sum by (le) (increase(rpc_server_duration_milliseconds_bucket{cluster="$cluster", namespace="$namespace", instance=~"$instance", rpc_service="opentelemetry.proto.collector.trace.v1.TraceService"}[$__rate_interval]))',
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),

      // "Batching" row
      (
        panel.new('Batching [otelcol.processor.batch]', 'row') +
        panel.withPosition({ h: 1, w: 24, x: 0, y: 10 })
      ),
      (
        panel.newHeatmap('Number of units in the batch') +
        panel.withDescription(|||
          Number of units in the batch
        |||) +
        panel.withPosition({ x: 0, y: 10, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr='sum by (le) (increase(processor_batch_batch_send_size_ratio_bucket{cluster="$cluster", namespace="$namespace", instance=~"$instance"}[$__rate_interval]))',
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),
      (
        panel.new(title='Distinct metadata values', type='timeseries') +
        panel.withDescription(|||
          Number of distinct metadata value combinations being processed
        |||) +
        panel.withPosition({ x: 8, y: 10, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
             processor_batch_metadata_cardinality_ratio{cluster="$cluster", namespace="$namespace", instance=~"$instance"}
            |||,
            legendFormat='{{ pod }}',
          ),
        ])
      ),
      (
        panel.new(title='Timeout trigger', type='timeseries') +
        panel.withDescription(|||
          Number of times the batch was sent due to a timeout trigger
        |||) +
        panel.withPosition({ x: 16, y: 10, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
             rate(processor_batch_timeout_trigger_send_ratio_total{cluster="$cluster", namespace="$namespace", instance=~"$instance"}[$__rate_interval])
            |||,
            legendFormat='{{ pod }}',
          ),
        ])
      ),

      // "Exporters for traces" row
      (
        panel.new('Exporters for traces [otelcol.exporter]', 'row') +
        panel.withPosition({ h: 1, w: 24, x: 0, y: 20 })
      ),
      (
        panel.new(title='Exported sent spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans successfully sent to destination.
        |||) +
        panel.withPosition({ x: 0, y: 20, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              rate(exporter_sent_spans_ratio_total{cluster="$cluster", namespace="$namespace", instance=~"$instance"}[$__rate_interval])
            |||,
            legendFormat='{{ pod }}',
          ),
        ])
      ),
      (
        panel.new(title='Exported failed spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans in failed attempts to send to destination.
        |||) +
        panel.withPosition({ x: 8, y: 20, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              rate(exporter_send_failed_spans_ratio_total{cluster="$cluster", namespace="$namespace", instance=~"$instance"}[$__rate_interval])
            |||,
            legendFormat='{{ pod }}',
          ),
        ])
      ),

    ]),
}
