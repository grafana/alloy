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

  local panelPosition(row, col) = panel.withPosition({x: col*8, y: row*10, w: 8, h: 10}),
  local rowPosition(row) = panel.withPosition({h: 1, w: 24, x: 0, y: row*10}),

  [filename]:
    dashboard.new(name='Alloy / OpenTelemetry', tag=$._config.dashboardTag) +
    dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin(templateVariables) +
    dashboard.withPanelsMixin([
      // "Receivers for metrics" row
      (
        panel.new('Receivers for metrics [otelcol.receiver]', 'row') +
        rowPosition(0)
      ),
      (
        panel.new(title='Accepted metric points', type='timeseries') +
        panel.withDescription(|||
          Number of metric points successfully pushed into the pipeline.
        |||) +
        stackedPanelMixin +
        panelPosition(row=0, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (rate(otelcol_receiver_accepted_metric_points_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Refused metric points', type='timeseries') +
        panel.withDescription(|||
          Number of metric points that could not be pushed into the pipeline.
        |||) +
        stackedPanelMixin +
        panelPosition(row=0, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (rate(otelcol_receiver_refused_metric_points_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.newHeatmap('RPC server duration for metrics', 'ms') +
        panel.withDescription(|||
          The duration of inbound RPCs for metrics.
        |||) +
        panelPosition(row=0, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(rpc_server_duration_milliseconds_bucket{%(instanceSelector)s, rpc_service="opentelemetry.proto.collector.metrics.v1.MetricsService"}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),

      // "Receivers for traces" row
      (
        panel.new('Receivers for traces [otelcol.receiver]', 'row') +
        rowPosition(1)
      ),
      (
        panel.new(title='Accepted spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans successfully pushed into the pipeline.
        |||) +
        stackedPanelMixin +
        panelPosition(row=1, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (rate(otelcol_receiver_accepted_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
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
        panelPosition(row=1, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (rate(otelcol_receiver_refused_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.newHeatmap('RPC server duration', 'ms') +
        panel.withDescription(|||
          The duration of inbound RPCs.
        |||) +
        panelPosition(row=1, col=2) +
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
        rowPosition(2)
      ),
      (
        panel.newHeatmap('Number of units in the batch', 'short') +
        panel.withUnit('short') +
        panel.withDescription(|||
          Number of spans, metric datapoints, or log lines in a batch
        |||) +
        panelPosition(row=2, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(otelcol_processor_batch_batch_send_size_bucket{%(instanceSelector)s}[$__rate_interval]))
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
        panelPosition(row=2, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (otelcol_processor_batch_metadata_cardinality{%(instanceSelector)s})
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Timeout trigger', type='timeseries') +
        panel.withDescription(|||
          Number of times the batch was sent due to a timeout trigger
        |||) +
        panelPosition(row=2, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (rate(otelcol_processor_batch_timeout_trigger_send_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),

      // "Exporters for traces" row
      (
        panel.new('Exporters for traces [otelcol.exporter]', 'row') +
        rowPosition(3)
      ),
      (
        panel.new(title='Exported sent spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans successfully sent to destination.
        |||) +
        stackedPanelMixin +
        panelPosition(row=3, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= ||| 
              sum by(instance) (rate(otelcol_exporter_sent_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Exported failed spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans in failed attempts to send to destination.
        |||) +
        stackedPanelMixin +
        panelPosition(row=3, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(instance) (rate(otelcol_exporter_send_failed_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
    ]),
}
