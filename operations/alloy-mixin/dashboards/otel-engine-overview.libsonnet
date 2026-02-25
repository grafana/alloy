local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local filename = 'alloy-otel-engine-overview.json';

{
  local templateVariables = [
    dashboard.newMultiTemplateVariable(
      'cluster',
      'label_values(otelcol_process_uptime_seconds_total, cluster)',
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels,
    ),
    dashboard.newMultiTemplateVariable(
      'namespace',
      'label_values(otelcol_process_uptime_seconds_total{cluster=~"$cluster"}, namespace)',
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels,
    ),
    dashboard.newMultiTemplateVariable(
      'job',
      'label_values(otelcol_process_uptime_seconds_total{cluster=~"$cluster", namespace=~"$namespace"}, job)',
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels,
    ),
    dashboard.newGroupByTemplateVariable(
      query='instance,receiver,transport,exporter,processor,otel_signal,otel_scope_name,job,namespace,cluster,pod',
      defaultValue='instance'
    ),
    {
      name: 'filters',
      type: 'adhoc',
      datasource: {
        type: 'prometheus',
        uid: '${datasource}',
      },
    },
  ],

  local panelPosition3Across(row, col) = panel.withPosition({ x: col * 8, y: row * 10, w: 8, h: 10 }),
  local rowPosition(row) = panel.withPosition({ h: 1, w: 24, x: 0, y: row * 10 }),

  local signalRow(
    title,
    rowNum,
    signalName,
    receiverAccepted,
    receiverRefused,
    exporterSent,
    exporterSendFailed,
    exporterEnqueueFailed,
    queueDataType,
  ) = (
    panel.new(title, 'row') +
    rowPosition(rowNum) +
    { collapsed: true } +
    {
      local rateQuery(metric) = ('rate(%s{%%(groupSelector)s}[$__rate_interval])' % metric) % $._config,

      panels: [
        (
          panel.new(title='Receiver: Total accepted & refused', type='timeseries') +
          panel.withDescription(
            'Total rate of accepted and refused %s across all receivers.' % signalName,
          ) +
          panel.withUnit('cps') +
          panelPosition3Across(row=rowNum, col=0) +
          panel.withQueries([
            panel.newQuery(
              expr='sum(%s)' % rateQuery(receiverAccepted),
              legendFormat='accepted',
            ),
            panel.newQuery(
              expr='sum(%s)' % rateQuery(receiverRefused),
              legendFormat='refused',
            ),
          ])
        ),
        (
          panel.new(title='Receiver: Accepted by ${groupby}', type='timeseries') +
          panel.withDescription(
            'Accepted and refused %s rates broken down by the selected dimension.' % signalName,
          ) +
          panel.withUnit('cps') +
          panelPosition3Across(row=rowNum, col=1) +
          panel.withQueries([
            panel.newQuery(
              expr='sum by(${groupby}) (%s)' % rateQuery(receiverAccepted),
              legendFormat='{{${groupby}}} accepted',
            ),
            panel.newQuery(
              expr='sum by(${groupby}) (%s)' % rateQuery(receiverRefused),
              legendFormat='{{${groupby}}} refused',
            ),
          ])
        ),
        (
          panel.new(title='Receiver: Refused by ${groupby}', type='timeseries') +
          panel.withDescription(
            'Refused %s rate broken down by the selected dimension.' % signalName,
          ) +
          panel.withUnit('cps') +
          panelPosition3Across(row=rowNum, col=2) +
          panel.withQueries([
            panel.newQuery(
              expr='sum by(${groupby}) (%s)' % rateQuery(receiverRefused),
              legendFormat='{{${groupby}}}',
            ),
          ])
        ),
        (
          panel.new(title='Exporter: Total sent & failed', type='timeseries') +
          panel.withDescription(
            'Total rate of sent, send-failed, and enqueue-failed %s across all exporters.' % signalName,
          ) +
          panel.withUnit('cps') +
          panelPosition3Across(row=rowNum + 1, col=0) +
          panel.withQueries([
            panel.newQuery(
              expr='sum(%s)' % rateQuery(exporterSent),
              legendFormat='sent',
            ),
            panel.newQuery(
              expr='sum(%s)' % rateQuery(exporterSendFailed),
              legendFormat='send failed',
            ),
            panel.newQuery(
              expr='sum(%s)' % rateQuery(exporterEnqueueFailed),
              legendFormat='enqueue failed',
            ),
          ])
        ),
        (
          panel.new(title='Exporter: Sent by ${groupby}', type='timeseries') +
          panel.withDescription(
            'Sent %s rate broken down by the selected dimension.' % signalName,
          ) +
          panel.withUnit('cps') +
          panelPosition3Across(row=rowNum + 1, col=1) +
          panel.withQueries([
            panel.newQuery(
              expr='sum by(${groupby}) (%s)' % rateQuery(exporterSent),
              legendFormat='{{${groupby}}} sent',
            ),
          ])
        ),
        (
          panel.new(title='Exporter: Failed by ${groupby}', type='timeseries') +
          panel.withDescription(
            'Send-failed and enqueue-failed %s rates broken down by the selected dimension.' % signalName,
          ) +
          panel.withUnit('cps') +
          panelPosition3Across(row=rowNum + 1, col=2) +
          panel.withQueries([
            panel.newQuery(
              expr='sum by(${groupby}) (%s)' % rateQuery(exporterSendFailed),
              legendFormat='{{${groupby}}} send failed',
            ),
            panel.newQuery(
              expr='sum by(${groupby}) (%s)' % rateQuery(exporterEnqueueFailed),
              legendFormat='{{${groupby}}} enqueue failed',
            ),
          ])
        ),
        (
          panel.new(title='Exporter: Queue utilization by ${groupby}', type='timeseries') +
          panel.withDescription(
            'Exporter send queue usage as a fraction of capacity. Values approaching 1 indicate backpressure and risk of data loss.',
          ) +
          panel.withUnit('percentunit') +
          panelPosition3Across(row=rowNum + 2, col=0) +
          panel.withQueries([
            panel.newQuery(
              expr=(|||
                sum by(${groupby}) (otelcol_exporter_queue_size{%%(groupSelector)s, data_type="%s"})
                /
                clamp_min(sum by(${groupby}) (otelcol_exporter_queue_capacity{%%(groupSelector)s, data_type="%s"}), 1)
              ||| % [queueDataType, queueDataType]) % $._config,
              legendFormat='{{${groupby}}}',
            ),
          ])
        ),
      ],
    }
  ),

  [filename]:
    dashboard.new(name='Alloy / OTel Engine Overview', tag=$._config.dashboardTag) +
    { description: 'Overview of the OpenTelemetry (OTel) engine running inside Alloy.' } +
    dashboard.withDocsLink(
      url='https://grafana.com/docs/alloy/latest/opentelemetry/',
      desc='OTel Engine documentation',
    ) +
    dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin(templateVariables) +
    dashboard.withPanelsMixin([
      // Overview row
      (
        panel.new('Overview', 'row') +
        rowPosition(0)
      ),
      (
        panel.newSingleStat('Pods count') {
          options+: { graphMode: 'area' },
        } +
        panel.withDescription(|||
          Current number of pods with OTel engine metrics.
        |||) +
        panel.withPosition({ x: 0, y: 0, w: 8, h: 5 }) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              count(otelcol_process_uptime_seconds_total{%(groupSelector)s})
            ||| % $._config,
            legendFormat='count',
          ),
        ])
      ),
      (
        panel.new(title='Recently started by ${groupby}', type='timeseries') +
        panel.withDescription(|||
          Count of series with process uptime under 60 seconds, grouped by the selected dimension.
        |||) +
        panel.withPosition({ x: 0, y: 5, w: 8, h: 5 }) +
        panel.withStacked(opacity=100, gradientMode='none') +
        panel.withDrawStyle('bars') +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              count by(${groupby}) (otelcol_process_uptime_seconds_total{%(groupSelector)s} < 60) or vector(0)
            ||| % $._config,
            legendFormat='{{${groupby}}}',
          ),
        ])
      ),
      (
        panel.new(title='Receivers SR', type='timeseries') +
        panel.withDescription(|||
          Receiver success rate for each signal type.
        |||) +
        panel.withUnit('percentunit') +
        panelPosition3Across(row=0, col=1) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              sum(rate(otelcol_receiver_accepted_spans_total{%(groupSelector)s}[$__rate_interval]))
              /
              (
                  sum(rate(otelcol_receiver_accepted_spans_total{%(groupSelector)s}[$__rate_interval]))
                  +
                  (sum(rate(otelcol_receiver_refused_spans_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
              )
            ||| % $._config,
            legendFormat='spans',
          ),
          panel.newQuery(
            expr=|||
              sum(rate(otelcol_receiver_accepted_metric_points_total{%(groupSelector)s}[$__rate_interval]))
              /
              (
                  sum(rate(otelcol_receiver_accepted_metric_points_total{%(groupSelector)s}[$__rate_interval]))
                  +
                  (sum(rate(otelcol_receiver_refused_metric_points_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
              )
            ||| % $._config,
            legendFormat='metric points',
          ),
          panel.newQuery(
            expr=|||
              sum(rate(otelcol_receiver_accepted_log_records_total{%(groupSelector)s}[$__rate_interval]))
              /
              (
                  sum(rate(otelcol_receiver_accepted_log_records_total{%(groupSelector)s}[$__rate_interval]))
                  +
                  (sum(rate(otelcol_receiver_refused_log_records_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
              )
            ||| % $._config,
            legendFormat='log records',
          ),
        ])
      ),
      (
        panel.new(title='Exporters SR', type='timeseries') +
        panel.withDescription(|||
          Exporter success rate for each signal type.
        |||) +
        panel.withUnit('percentunit') +
        panelPosition3Across(row=0, col=2) +
        panel.withQueries([
          panel.newQuery(
            expr=|||
              sum(rate(otelcol_exporter_sent_spans_total{%(groupSelector)s}[$__rate_interval]))
              /
              (
                  sum(rate(otelcol_exporter_sent_spans_total{%(groupSelector)s}[$__rate_interval]))
                  +
                  (sum(rate(otelcol_exporter_enqueue_failed_spans_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
                  +
                  (sum(rate(otelcol_exporter_send_failed_spans_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
              )
            ||| % $._config,
            legendFormat='spans',
          ),
          panel.newQuery(
            expr=|||
              sum(rate(otelcol_exporter_sent_metric_points_total{%(groupSelector)s}[$__rate_interval]))
              /
              (
                  sum(rate(otelcol_exporter_sent_metric_points_total{%(groupSelector)s}[$__rate_interval]))
                  +
                  (sum(rate(otelcol_exporter_enqueue_failed_metric_points_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
                  +
                  (sum(rate(otelcol_exporter_send_failed_metric_points_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
              )
            ||| % $._config,
            legendFormat='metric points',
          ),
          panel.newQuery(
            expr=|||
              sum(rate(otelcol_exporter_sent_log_records_total{%(groupSelector)s}[$__rate_interval]))
              /
              (
                  sum(rate(otelcol_exporter_sent_log_records_total{%(groupSelector)s}[$__rate_interval]))
                  +
                  (sum(rate(otelcol_exporter_enqueue_failed_log_records_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
                  +
                  (sum(rate(otelcol_exporter_send_failed_log_records_total{%(groupSelector)s}[$__rate_interval])) or vector(0))
              )
            ||| % $._config,
            legendFormat='log records',
          ),
        ])
      ),

      // Signal-specific rows (collapsed by default)
      signalRow(
        title='Spans & Traces',
        rowNum=2,
        signalName='spans',
        receiverAccepted='otelcol_receiver_accepted_spans_total',
        receiverRefused='otelcol_receiver_refused_spans_total',
        exporterSent='otelcol_exporter_sent_spans_total',
        exporterSendFailed='otelcol_exporter_send_failed_spans_total',
        exporterEnqueueFailed='otelcol_exporter_enqueue_failed_spans_total',
        queueDataType='traces',
      ),
      signalRow(
        title='Metrics',
        rowNum=5,
        signalName='metric points',
        receiverAccepted='otelcol_receiver_accepted_metric_points_total',
        receiverRefused='otelcol_receiver_refused_metric_points_total',
        exporterSent='otelcol_exporter_sent_metric_points_total',
        exporterSendFailed='otelcol_exporter_send_failed_metric_points_total',
        exporterEnqueueFailed='otelcol_exporter_enqueue_failed_metric_points_total',
        queueDataType='metrics',
      ),
      signalRow(
        title='Logs',
        rowNum=8,
        signalName='log records',
        receiverAccepted='otelcol_receiver_accepted_log_records_total',
        receiverRefused='otelcol_receiver_refused_log_records_total',
        exporterSent='otelcol_exporter_sent_log_records_total',
        exporterSendFailed='otelcol_exporter_send_failed_log_records_total',
        exporterEnqueueFailed='otelcol_exporter_enqueue_failed_log_records_total',
        queueDataType='logs',
      ),

      // Processing & Batching row (collapsed by default)
      (
        panel.new('Processing & Batching', 'row') +
        rowPosition(11) +
        { collapsed: true } +
        {
          local normalNote = 'Requires "normal" telemetry level.',

          panels: [
            // Row 1: Processor throughput
            (
              panel.new(title='Processor throughput', type='timeseries') +
              panel.withDescription(
                'Incoming vs outgoing items across all processors. A gap between the two may indicate items being dropped or filtered.',
              ) +
              panel.withUnit('cps') +
              panelPosition3Across(row=11, col=0) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum(rate(otelcol_processor_incoming_items_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='incoming',
                ),
                panel.newQuery(
                  expr=|||
                    sum(rate(otelcol_processor_outgoing_items_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='outgoing',
                ),
              ])
            ),
            (
              panel.new(title='Processor throughput by ${groupby}', type='timeseries') +
              panel.withDescription(
                'Incoming vs outgoing items broken down by the selected dimension.',
              ) +
              panel.withUnit('cps') +
              panelPosition3Across(row=11, col=1) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (rate(otelcol_processor_incoming_items_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='{{${groupby}}} incoming',
                ),
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (rate(otelcol_processor_outgoing_items_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='{{${groupby}}} outgoing',
                ),
              ])
            ),
            (
              panel.new(title='Processor refused by ${groupby}', type='timeseries') +
              panel.withDescription(
                'Items refused by processors, broken down by signal type and selected dimension.',
              ) +
              panel.withUnit('cps') +
              panelPosition3Across(row=11, col=2) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (rate(otelcol_processor_refused_spans_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='{{${groupby}}} spans',
                ),
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (rate(otelcol_processor_refused_metric_points_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='{{${groupby}}} metric points',
                ),
              ])
            ),
            // Row 2: Batching
            (
              panel.newHeatmap('Batch send size', 'short') +
              panel.withDescription(
                'Distribution of batch sizes when sent. Shows how full batches are before being flushed. ' + normalNote,
              ) +
              panelPosition3Across(row=12, col=0) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum by (le) (increase(otelcol_processor_batch_batch_send_size_bucket{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  format='heatmap',
                  legendFormat='{{le}}',
                ),
              ])
            ),
            (
              panel.new(title='Batch send triggers by ${groupby}', type='timeseries') +
              panel.withDescription(
                'How batches are flushed: by reaching the size limit or by timeout. Mostly timeout triggers may indicate low throughput or a large batch size setting. ' + normalNote,
              ) +
              panel.withUnit('cps') +
              panelPosition3Across(row=12, col=1) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (rate(otelcol_processor_batch_batch_size_trigger_send_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='{{${groupby}}} size trigger',
                ),
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (rate(otelcol_processor_batch_timeout_trigger_send_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='{{${groupby}}} timeout trigger',
                ),
              ])
            ),
            (
              panel.new(title='Batch metadata cardinality by ${groupby}', type='timeseries') +
              panel.withDescription(
                'Number of distinct metadata value combinations being processed. High cardinality increases memory usage and may hit the metadata_cardinality_limit. ' + normalNote,
              ) +
              panelPosition3Across(row=12, col=2) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (otelcol_processor_batch_metadata_cardinality{%(groupSelector)s})
                  ||| % $._config,
                  legendFormat='{{${groupby}}}',
                ),
              ])
            ),
          ],
        }
      ),

      // Process Resources row (collapsed by default)
      (
        panel.new('Process Resources', 'row') +
        rowPosition(13) +
        { collapsed: true } +
        {
          local avgLineStyle = [
            { id: 'color', value: { fixedColor: 'green', mode: 'fixed' } },
            { id: 'custom.lineStyle', value: { dash: [10, 10], fill: 'dash' } },
            { id: 'custom.lineWidth', value: 2 },
          ],

          panels: [
            (
              panel.new(title='CPU usage', type='timeseries') +
              panel.withDescription(
                'CPU cores used by OTel engine processes. 1.0 = 100% of one core.',
              ) +
              panel.withUnit('short') +
              panelPosition3Across(row=13, col=0) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    avg(sum by(instance) (rate(otelcol_process_cpu_seconds_total{%(groupSelector)s}[$__rate_interval])))
                  ||| % $._config,
                  legendFormat='average',
                ),
                panel.newQuery(
                  expr=|||
                    sum by(instance) (rate(otelcol_process_cpu_seconds_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                ),
              ]) +
              panel.withOverridesByName('average', avgLineStyle)
            ),
            (
              panel.new(title='Memory RSS', type='timeseries') +
              panel.withDescription(
                'Resident set size (physical memory) used by OTel engine processes.',
              ) +
              panel.withUnit('bytes') +
              panelPosition3Across(row=13, col=1) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    avg(sum by(instance) (otelcol_process_memory_rss_bytes{%(groupSelector)s}))
                  ||| % $._config,
                  legendFormat='average',
                ),
                panel.newQuery(
                  expr=|||
                    sum by(instance) (otelcol_process_memory_rss_bytes{%(groupSelector)s})
                  ||| % $._config,
                ),
              ]) +
              panel.withOverridesByName('average', avgLineStyle)
            ),
            (
              panel.new(title='Go allocation rate', type='timeseries') +
              panel.withDescription(
                'Rate of heap memory allocation. High allocation rates often lead to GC pressure.',
              ) +
              panel.withUnit('Bps') +
              panelPosition3Across(row=13, col=2) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    avg(sum by(instance) (rate(otelcol_process_runtime_total_alloc_bytes_total{%(groupSelector)s}[$__rate_interval])))
                  ||| % $._config,
                  legendFormat='average',
                ),
                panel.newQuery(
                  expr=|||
                    sum by(instance) (rate(otelcol_process_runtime_total_alloc_bytes_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                ),
              ]) +
              panel.withOverridesByName('average', avgLineStyle)
            ),
          ],
        }
      ),

      // Grafana Cloud connector row (collapsed by default)
      (
        panel.new('Grafana Cloud Connector', 'row') +
        rowPosition(15) +
        { collapsed: true } +
        {
          local connectorNote = 'Only available when the grafanacloud connector is configured.',

          panels: [
            (
              panel.new(title='Unique hosts tracked', type='timeseries') +
              panel.withDescription(
                'Number of unique hosts identified by the grafanacloud connector for Application Observability billing. ' + connectorNote,
              ) +
              panelPosition3Across(row=15, col=0) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum by(${groupby}) (otelcol_grafanacloud_host_count_ratio{%(groupSelector)s})
                  ||| % $._config,
                  legendFormat='{{${groupby}}}',
                ),
              ])
            ),
            (
              panel.new(title='Datapoints sent rate', type='timeseries') +
              panel.withDescription(
                'Total rate of datapoints sent to Grafana Cloud by the grafanacloud connector. ' + connectorNote,
              ) +
              panel.withUnit('cps') +
              panelPosition3Across(row=15, col=1) +
              panel.withQueries([
                panel.newQuery(
                  expr=|||
                    sum(rate(otelcol_grafanacloud_datapoint_count_total{%(groupSelector)s}[$__rate_interval]))
                  ||| % $._config,
                  legendFormat='total',
                ),
              ])
            ),
          ],
        }
      ),
    ]),
}
