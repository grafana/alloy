local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local templates = import './utils/templates.libsonnet';
local filename = 'alloy-opentelemetry.json';

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
  local templateVariables = 
    templates.newTemplateVariablesList(
      filterSelector=$._config.filterSelector, 
      enableK8sCluster=$._config.enableK8sCluster, 
      includeInstance=true,
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),

  [filename]:
    dashboard.new(name='Alloy / OpenTelemetry', tag=$._config.dashboardTag) +
    dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin(templateVariables) +
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
        stackedPanelMixin +
        panel.withPosition({ x: 0, y: 0, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              rate(receiver_accepted_spans_ratio_total{%(instanceSelector)s}[$__rate_interval])
            ||| % $._config,
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
        stackedPanelMixin +
        panel.withPosition({ x: 8, y: 0, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              rate(receiver_refused_spans_ratio_total{%(instanceSelector)s}[$__rate_interval])
            ||| % $._config,
            legendFormat='{{ pod }} / {{ transport }}',
          ),
        ])
      ),
      (
        panel.newHeatmap('RPC server duration', 'ms') +
        panel.withDescription(|||
          The duration of inbound RPCs.
        |||) +
        panel.withPosition({ x: 16, y: 0, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(rpc_server_duration_milliseconds_bucket{%(instanceSelector)s, rpc_service="opentelemetry.proto.collector.trace.v1.TraceService"}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),

      // "Batching" row
      (
        panel.new('Batching of logs, metrics, and traces [otelcol.processor.batch]', 'row') +
        panel.withPosition({ h: 1, w: 24, x: 0, y: 10 })
      ),
      (
        panel.newHeatmap('Number of units in the batch', 'short') +
        panel.withUnit('short') +
        panel.withDescription(|||
          Number of spans, metric datapoints, or log lines in a batch
        |||) +
        panel.withPosition({ x: 0, y: 10, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(processor_batch_batch_send_size_ratio_bucket{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),
      (
        panel.new(title='Distinct metadata values', type='timeseries') +
        //TODO: Clarify what metadata means. I think it's the metadata in the HTTP headers?
        //TODO: Mention that if this metric is too high, it could hit the metadata_cardinality_limit
        //TODO: MAke a metric for the current value of metadata_cardinality_limit and create an alert if the actual cardinality reaches it?
        panel.withDescription(|||
          Number of distinct metadata value combinations being processed
        |||) +
        panel.withPosition({ x: 8, y: 10, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              processor_batch_metadata_cardinality_ratio{%(instanceSelector)s}
            ||| % $._config,
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
            expr= |||
              rate(processor_batch_timeout_trigger_send_ratio_total{%(instanceSelector)s}[$__rate_interval])
            ||| % $._config,
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
        stackedPanelMixin +
        panel.withPosition({ x: 0, y: 20, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= ||| 
              rate(exporter_sent_spans_ratio_total{%(instanceSelector)s}[$__rate_interval])
            ||| % $._config,
            legendFormat='{{ pod }}',
          ),
        ])
      ),
      (
        panel.new(title='Exported failed spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans in failed attempts to send to destination.
        |||) +
        stackedPanelMixin +
        panel.withPosition({ x: 8, y: 20, w: 8, h: 10 }) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              rate(exporter_send_failed_spans_ratio_total{%(instanceSelector)s}[$__rate_interval])
            ||| % $._config,
            legendFormat='{{ pod }}',
          ),
        ])
      ),

    ]),
}
