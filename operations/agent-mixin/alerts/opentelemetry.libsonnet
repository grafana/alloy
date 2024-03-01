local alert = import './utils/alert.jsonnet';

alert.newGroup(
  'otelcol',
  [
    // An otelcol.exporter component rcould not push some spans to the pipeline.
    // This could be due to reaching a limit such as the ones
    // imposed by otelcol.processor.memory_limiter.
    alert.newRule(
      'OtelcolReceiverRefusedSpans',
      'sum(rate(receiver_refused_spans_ratio_total{}[1m])) > 0',
      'The receiver could not push some spans to the pipeline.',
      '5m',
    ),

    // The exporter failed to send spans to their destination.
    // There could be an issue with the payload or with the destination endpoint.
    alert.newRule(
      'OtelcolExporterFailedSpans',
      'sum(rate(exporter_send_failed_spans_ratio_total{}[1m])) > 0',
      'The exporter failed to send spans to their destination.',
      '5m',
    ),
  ]
)
