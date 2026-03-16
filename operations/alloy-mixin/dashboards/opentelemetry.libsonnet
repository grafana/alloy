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
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels
    ) + [
      dashboard.newGroupByTemplateVariable(
        query='instance,receiver,transport,exporter,processor,component_id,otel_signal,otel_scope_name,job,namespace,cluster,pod',
        defaultValue='instance'
      ),
    ],

  local panelPosition3Across(row, col) = panel.withPosition({x: col*8, y: row*10, w: 8, h: 10}),
  local panelPosition4Across(row, col) = panel.withPosition({x: col*6, y: row*10, w: 6, h: 10}),
  local rowPosition(row) = panel.withPosition({h: 1, w: 24, x: 0, y: row*10}),

  [filename]:
    dashboard.new(name='Alloy / OpenTelemetry', tag=$._config.dashboardTag) +
    dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin(templateVariables) +
    dashboard.withPanelsMixin([
      // First row - Metrics and Logs
      (
        panel.new('Receivers [otelcol.receiver.*]', 'row') +
        rowPosition(0)
      ),
      (
        panel.new(title='Accepted metric points', type='timeseries') +
        panel.withDescription(|||
          Number of metric points successfully pushed into the pipeline.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=0, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_receiver_accepted_metric_points_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Refused metric points', type='timeseries') +
        panel.withDescription(|||
          Number of metric points that could not be pushed into the pipeline.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=0, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_receiver_refused_metric_points_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Accepted logs', type='timeseries') +
        panel.withDescription(|||
          Number of log records successfully pushed into the pipeline.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=0, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_receiver_accepted_log_records_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Refused logs', type='timeseries') +
        panel.withDescription(|||
          Number of log records that could not be pushed into the pipeline.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=0, col=3) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_receiver_refused_log_records_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Accepted spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans successfully pushed into the pipeline.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=1, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_receiver_accepted_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Refused spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans that could not be pushed into the pipeline.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=1, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_receiver_refused_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.newHeatmap('RPC server duration', 'ms') +
        panel.withDescription(|||
          The duration of inbound RPCs for otelcol.receiver.* components.
        |||) +
        panelPosition4Across(row=1, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(rpc_server_duration_milliseconds_bucket{%(instanceSelector)s, component_id=~"otelcol.receiver.*"}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),
      (
        panel.newHeatmap('HTTP server duration', 'ms') +
        panel.withDescription(|||
          The duration of inbound HTTP requests for otelcol.receiver.* components.
        |||) +
        panelPosition4Across(row=1, col=3) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(http_server_duration_milliseconds_bucket{%(instanceSelector)s, component_id=~"otelcol.receiver.*"}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),

      // "Batching" row
      (
        panel.new('Batching [otelcol.processor.batch]', 'row') +
        rowPosition(2)
      ),
      (
        panel.newHeatmap('Number of units in the batch', 'short') +
        panel.withUnit('short') +
        panel.withDescription(|||
          Number of spans, metric datapoints, or log lines in a batch
        |||) +
        panelPosition3Across(row=2, col=0) +
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
        panel.withUnit('short') +
        panelPosition3Across(row=2, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (otelcol_processor_batch_metadata_cardinality{%(instanceSelector)s})
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Timeout trigger', type='timeseries') +
        panel.withDescription(|||
          Number of times the batch was sent due to a timeout trigger
        |||) +
        panel.withUnit('cps') +
        panelPosition3Across(row=2, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_processor_batch_timeout_trigger_send_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),

      // "Exporters" row
      (
        panel.new('Exporters [otelcol.exporter.*]', 'row') +
        rowPosition(3)
      ),
      (
        panel.new(title='Exported metric points', type='timeseries') +
        panel.withDescription(|||
          Number of metric points successfully sent to destination.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=3, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= ||| 
              sum by(${groupby}) (rate(otelcol_exporter_sent_metric_points_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Failed metric points', type='timeseries') +
        panel.withDescription(|||
          Number of metric points that failed to be sent to destination.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=3, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_exporter_send_failed_metric_points_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Exported logs', type='timeseries') +
        panel.withDescription(|||
          Number of log records successfully sent to destination.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=3, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_exporter_sent_log_records_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Failed logs', type='timeseries') +
        panel.withDescription(|||
          Number of log records that failed to be sent to destination.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=3, col=3) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_exporter_send_failed_log_records_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Exported spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans successfully sent to destination.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=4, col=0) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_exporter_sent_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.new(title='Failed spans', type='timeseries') +
        panel.withDescription(|||
          Number of spans that failed to be sent to destination.
        |||) +
        panel.withUnit('cps') +
        stackedPanelMixin +
        panelPosition4Across(row=4, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by(${groupby}) (rate(otelcol_exporter_send_failed_spans_total{%(instanceSelector)s}[$__rate_interval]))
            ||| % $._config,
          ),
        ])
      ),
      (
        panel.newHeatmap('RPC client duration', 'ms') +
        panel.withDescription(|||
          The duration of outbound RPCs for otelcol.exporter.* components.
        |||) +
        panelPosition4Across(row=4, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(rpc_client_duration_milliseconds_bucket{%(instanceSelector)s, component_id=~"otelcol.exporter.*"}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),
      (
        panel.newHeatmap('HTTP client duration', 'ms') +
        panel.withDescription(|||
          The duration of outbound HTTP requests for otelcol.exporter.* components.
        |||) +
        panelPosition4Across(row=4, col=3) +
        panel.withQueries([
          panel.newQuery(
            expr= |||
              sum by (le) (increase(http_client_duration_milliseconds_bucket{%(instanceSelector)s, component_id=~"otelcol.exporter.*"}[$__rate_interval]))
            ||| % $._config,
            format='heatmap',
            legendFormat='{{le}}',
          ),
        ])
      ),
    ]),
}
