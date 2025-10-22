local alert = import './utils/alert.jsonnet';

{
  local successThreshold = 0.95,
  local successThresholdText = '95%',
  local pendingPeriod = '10m',

  local successRateQuery(enableK8sCluster, failed, success) =
        local sumBy = if enableK8sCluster then "cluster, namespace, job" else "job";
        |||
          (1 - (
                  sum by (%s) (rate(%s{}[1m]))
                  /
                  sum by (%s) (rate(%s{}[1m]) + rate(%s{}[1m]))
               )
          ) < %g
        ||| % [sumBy, failed, sumBy, failed, success, successThreshold],

  newOpenTelemetryAlertsGroup(enableK8sCluster=true):
    alert.newGroup(
      'alloy_otelcol',
      [
        // An otelcol.receiver component could not push over 5% of spans to the pipeline.
        // This could be due to reaching a limit such as the ones
        // imposed by otelcol.processor.memory_limiter.
        alert.newRule(
          'OtelcolReceiverRefusedSpans',
          successRateQuery(enableK8sCluster, "otelcol_receiver_refused_spans_total", "otelcol_receiver_accepted_spans_total"),
          'The receiver pushing spans to the pipeline success rate is below %s.' % successThresholdText,
          'The receiver could not push some spans to the pipeline under job {{ $labels.job }}. This could be due to reaching a limit such as the ones imposed by otelcol.processor.memory_limiter.',
          pendingPeriod,
        ),

        // Metrics receiver alerts
        alert.newRule(
          'OtelcolReceiverRefusedMetrics',
          successRateQuery(enableK8sCluster, "otelcol_receiver_refused_metric_points_total", "otelcol_receiver_accepted_metric_points_total"),
          'The receiver pushing metrics to the pipeline success rate is below %s.' % successThresholdText,
          'The receiver could not push some metric points to the pipeline under job {{ $labels.job }}. This could be due to reaching a limit such as the ones imposed by otelcol.processor.memory_limiter.',
          pendingPeriod,
        ),

        // Logs receiver alerts
        alert.newRule(
          'OtelcolReceiverRefusedLogs',
          successRateQuery(enableK8sCluster, "otelcol_receiver_refused_log_records_total", "otelcol_receiver_accepted_log_records_total"),
          'The receiver pushing logs to the pipeline success rate is below %s.' % successThresholdText,
          'The receiver could not push some log records to the pipeline under job {{ $labels.job }}. This could be due to reaching a limit such as the ones imposed by otelcol.processor.memory_limiter.',
          pendingPeriod,
        ),

        // The exporter success rate is below threshold.
        // There could be an issue with the payload or with the destination endpoint.
        alert.newRule(
          'OtelcolExporterFailedSpans',
          successRateQuery(enableK8sCluster, "otelcol_exporter_send_failed_spans_total", "otelcol_exporter_sent_spans_total"),
          'The exporter sending spans success rate is below %s.' % successThresholdText,
          'The exporter failed to send spans to their destination under job {{ $labels.job }}. There could be an issue with the payload or with the destination endpoint.',
          pendingPeriod,
        ),

        // Metrics exporter alerts
        alert.newRule(
          'OtelcolExporterFailedMetrics',
          successRateQuery(enableK8sCluster, "otelcol_exporter_send_failed_metric_points_total", "otelcol_exporter_sent_metric_points_total"),
          'The exporter sending metrics success rate is below %s.' % successThresholdText,
          'The exporter failed to send metric points to their destination under job {{ $labels.job }}. There could be an issue with the payload or with the destination endpoint.',
          pendingPeriod,
        ),

        // Logs exporter alerts
        alert.newRule(
          'OtelcolExporterFailedLogs',
          successRateQuery(enableK8sCluster, "otelcol_exporter_send_failed_log_records_total", "otelcol_exporter_sent_log_records_total"),
          'The exporter sending logs success rate is below %s.' % successThresholdText,
          'The exporter failed to send log records to their destination under job {{ $labels.job }}. There could be an issue with the payload or with the destination endpoint.',
          pendingPeriod,
        ),
      ]
    )
}
