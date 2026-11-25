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
        // The stream_limited and queue_is_full drops come from limits that the user sets, so they don't fire the alert.
        alert.newRule(
          'LokiWriteDroppedEntries',
          'sum by (%s) (increase(loki_write_dropped_entries_total{reason!~"stream_limited|queue_is_full"}[10m])) > 0' % sumBy(enableK8sCluster, ['host', 'reason']),
          'loki.write is dropping log entries.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} dropped log entries for host {{ $labels.host }} with reason {{ $labels.reason }}.',
          '5m',
        ),

        // loki.write retries a failed WAL write until it succeeds, so the pipeline blocks while writes fail.
        // A single failure that a retry fixes doesn't fire the alert, because the 5m increase drops to zero before the 10m pending period ends.
        // The instance label identifies the Alloy instance with the failing disk.
        alert.newRule(
          'LokiWriteWALWriteFailures',
          'sum by (%s) (increase(loki_write_wal_writer_failures_total[5m])) > 0' % sumBy(enableK8sCluster, ['instance']),
          'loki.write is failing to write log entries to the WAL.',
          'loki.write component {{ $labels.component_id }} on instance {{ $labels.instance }} under job {{ $labels.job }} has failed to write log entries to the WAL for 10 minutes. loki.write blocks the pipeline while the writes fail. Check the free disk space for the WAL directory.',
          '10m',
        ),

        // Each WAL record holds the stream of its entries, so a stream is missing only when the WAL cleanup
        // removed it before the reader got to its entries.
        alert.newRule(
          'LokiWriteWALStreamNotFound',
          'sum by (%s) (increase(loki_write_wal_entries_stream_not_found_total[10m])) > 0' % sumBy(enableK8sCluster, ['id']),
          'loki.write is dropping WAL entries because the WAL reader is behind.',
          'loki.write component {{ $labels.component_id }} under job {{ $labels.job }} dropped log entries read from the WAL for endpoint {{ $labels.id }}. The WAL reader is behind by more than max_segment_age, so log entries in removed WAL segments are likely lost.',
          '5m',
        ),

        // Retries are normal during short failures and on endpoints that rate-limit,
        // so the alert fires only when retries continue and no entries get through.
        // The 15m window is longer than the default 5m max_backoff_period, so slow retries stay visible.
        // The instance label stops healthy instances from hiding an instance that can't send.
        alert.newRule(
          'LokiWriteSustainedRetries',
          |||
            sum by (%(by)s) (increase(loki_write_batch_retries_total[15m])) > 0
            and
            sum by (%(by)s) (increase(loki_write_sent_entries_total[15m])) == 0
          ||| % { by: sumBy(enableK8sCluster, ['instance', 'host']) },
          'loki.write is continuously retrying batches.',
          'loki.write component {{ $labels.component_id }} on instance {{ $labels.instance }} under job {{ $labels.job }} has retried batches to host {{ $labels.host }} for at least 20 minutes without sending any entries. The endpoint may be unreachable.',
          '5m',
        ),
      ]
    ),
}
