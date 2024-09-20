local alert = import './utils/alert.jsonnet';

{
  local successRateQuery(enableK8sCluster, failed, success) =
        local sumBy = if enableK8sCluster then "cluster, namespace, job" else "job";
        |||
          (1 - sum by (%s) (
                  rate(%s{}[1m])
                  /
                  (rate(%s{}[1m]) + rate(%s{}[1m]))
               )
          ) < 0.95
        ||| % [sumBy, failed, failed, success],

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
          'The receiver pushing spans to the pipeline success rate is below 95%.',
          'The receiver could not push some spans to the pipeline under job {{ $labels.job }}. This could be due to reaching a limit such as the ones imposed by otelcol.processor.memory_limiter.',
          '10m',
        ),

        // The exporter success rate is below 95%.
        // There could be an issue with the payload or with the destination endpoint.
        alert.newRule(
          'OtelcolExporterFailedSpans',
          successRateQuery(enableK8sCluster, "otelcol_exporter_send_failed_spans_total", "otelcol_exporter_sent_spans_total"),
          'The exporter sending spans success rate is below 95%.',
          'The exporter failed to send spans to their destination under job {{ $labels.job }}. There could be an issue with the payload or with the destination endpoint.',
          '10m',
        ),
      ]
    )
}
