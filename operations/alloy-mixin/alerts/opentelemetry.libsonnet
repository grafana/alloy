local alert = import './utils/alert.jsonnet';

{
  newOpenTelemetryAlertsGroup(enableK8sCluster=true, percentThreshold='5', forT='5m'):
    alert.newGroup(
      'alloy_otelcol',
      [
        // An otelcol.exporter component rcould not push some spans to the pipeline.
        // This could be due to reaching a limit such as the ones
        // imposed by otelcol.processor.memory_limiter.
        alert.newRule(
          'OtelcolReceiverRefusedSpans',
          if enableK8sCluster then
            '(
                sum by (cluster, namespace, job) (rate(receiver_refused_spans_ratio_total{}[5m])) / 
                (
                    sum by (cluster, namespace, job) (rate(receiver_refused_spans_ratio_total{}[5m])) 
                    + 
                    sum by (cluster, namespace, job) (rate(receiver_accepted_spans_ratio_total{}[5m]))
                )
            ) * 100 > %s' % percentThreshold
          else
            '(
                sum by (job) (rate(receiver_refused_spans_ratio_total{}[5m])) / 
                (
                    sum by (job) (rate(receiver_refused_spans_ratio_total{}[5m])) 
                    + 
                    sum by (job) (rate(receiver_accepted_spans_ratio_total{}[5m]))
                )
            ) * 100 > %s' % percentThreshold
          ,
          'The receiver could not push some spans to the pipeline.',
          'The receiver could not push some spans to the pipeline under job {{ $labels.job }}. This could be due to reaching a limit such as the ones imposed by otelcol.processor.memory_limiter.',
          forT,
        ),

        // The exporter failed to send spans to their destination.
        // There could be an issue with the payload or with the destination endpoint.
        alert.newRule(
          'OtelcolExporterFailedSpans',
          if enableK8sCluster then
            '(
                sum by (cluster, namespace, job) (rate(exporter_send_failed_spans_ratio_total{}[5m])) / 
                (
                    sum by (cluster, namespace, job) (rate(exporter_send_failed_spans_ratio_total{}[5m])) 
                    + 
                    sum by (cluster, namespace, job) (rate(exporter_sent_spans_ratio_total{}[5m]))
                )
            ) * 100 > %s' % percentThreshold
          else
            '(
                sum by (job) (rate(exporter_send_failed_spans_ratio_total{}[5m])) / 
                (
                    sum by (job) (rate(exporter_send_failed_spans_ratio_total{}[5m])) 
                    + 
                    sum by (job) (rate(exporter_sent_spans_ratio_total{}[5m]))
                )
            ) * 100 > %s' % percentThreshold
          ,
          'The exporter failed to send spans to their destination.',
          'The exporter failed to send spans to their destination under job {{ $labels.job }}. There could be an issue with the payload or with the destination endpoint.',
          forT,
        ),
      ]
    )
}
