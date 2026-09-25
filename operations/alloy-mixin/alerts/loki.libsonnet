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

        // A failed WAL write loses the entry, for example when the disk is full.
        alert.newRule(
          'LokiWriteWALWriteFailures',
          'sum by (%s) (increase(loki_write_wal_writer_failed_entries_total[10m])) > 0' % sumBy(enableK8sCluster),
          'loki.write is failing to write log entries to the WAL.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} failed to write log entries to the WAL. Check the free disk space for the WAL directory.',
          '5m',
        ),

        alert.newRule(
          'LokiWriteWALStreamNotFound',
          'sum by (%s) (increase(loki_write_wal_entries_stream_not_found_total[10m])) > 0' % sumBy(enableK8sCluster, ['id']),
          'loki.write is dropping WAL entries with an unknown stream.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} dropped log entries read from the WAL for endpoint {{ $labels.id }} because their stream was not found.',
          '5m',
        ),

        // Retries are normal during short failures, so only a sustained retry rate fires the alert.
        alert.newRule(
          'LokiWriteSustainedRetries',
          'sum by (%s) (rate(loki_write_batch_retries_total[5m])) > 0' % sumBy(enableK8sCluster, ['host']),
          'loki.write is continuously retrying batches.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} has retried batches to host {{ $labels.host }} for 15 minutes. The endpoint may be unreachable.',
          '15m',
        ),
      ]
    ),
}
