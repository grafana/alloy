local alert = import './utils/alert.jsonnet';

{
  local sumBy(enableK8sCluster, extra=[]) =
    std.join(', ', (if enableK8sCluster then ['cluster', 'namespace', 'job'] else ['job']) + ['component_path', 'component_id'] + extra),

  newLokiAlertsGroup(enableK8sCluster=true):
    alert.newGroup(
      'alloy_loki',
      [
        // A healthy pipeline never drops entries.
        // A single drop keeps the 10m increase above zero for longer than the 5m pending period, so it fires the alert.
        alert.newRule(
          'LokiWriteDroppedEntries',
          'sum by (%s) (increase(loki_write_dropped_entries_total[10m])) > 0' % sumBy(enableK8sCluster, ['host', 'reason']),
          'loki.write is dropping log entries.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} dropped log entries for host {{ $labels.host }} with reason {{ $labels.reason }}.',
          '5m',
        ),

        // loki.write retries a failed WAL write until it succeeds, so the pipeline blocks while writes fail.
        alert.newRule(
          'LokiWriteWALWriteFailures',
          'sum by (%s) (increase(loki_write_wal_writer_failures_total[10m])) > 0' % sumBy(enableK8sCluster),
          'loki.write is failing to write log entries to the WAL.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} failed to write log entries to the WAL and blocks the pipeline while it retries. Check the free disk space for the WAL directory.',
          '5m',
        ),

        alert.newRule(
          'LokiWriteWALStreamNotFound',
          'sum by (%s) (increase(loki_write_wal_entries_stream_not_found_total[10m])) > 0' % sumBy(enableK8sCluster, ['id']),
          'loki.write is dropping WAL entries with an unknown stream.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} dropped log entries read from the WAL for endpoint {{ $labels.id }} because their stream was not found.',
          '5m',
        ),

        // Retries are normal during short failures and on endpoints that rate-limit,
        // so the alert fires only when retries continue and no entries get through.
        alert.newRule(
          'LokiWriteSustainedRetries',
          |||
            sum by (%(by)s) (rate(loki_write_batch_retries_total[5m])) > 0
            and
            sum by (%(by)s) (rate(loki_write_sent_entries_total[5m])) == 0
          ||| % { by: sumBy(enableK8sCluster, ['host']) },
          'loki.write is continuously retrying batches.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} has retried batches to host {{ $labels.host }} for 15 minutes without sending any entries. The endpoint may be unreachable.',
          '15m',
        ),
      ]
    ),
}
