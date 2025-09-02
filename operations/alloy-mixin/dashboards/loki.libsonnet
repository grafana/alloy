local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local templates = import './utils/templates.libsonnet';
local filename = 'alloy-loki.json';

{
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
  },

  local sourceFilePanels(y_offset) = [
    panel.newRow(title='loki.source.file', y=y_offset),

    // Active files being tailed
    (
      panel.new(title='Active files count $cluster', type='timeseries') +
      panel.withUnit('files') +
      panel.withDescription(|||
        Active files being tailed.
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by($groupBy) (loki_source_file_files_active_total{cluster=~"$cluster", namespace=~"$namespace"})
          |||,
        ),
      ])
      + stackedPanelMixin
    ),
  ],

  local journalFilePanels(y_offset) = [
    panel.newRow(title='loki.source.journal', y=y_offset),

    // Journal lines
    (
      panel.new(title='Journal lines read in $cluster', type='timeseries') +
      panel.withUnit('cps') +
      panel.withDescription(|||
        Successful journal reads.	
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by ($groupBy) (rate(loki_source_journal_target_lines_total{cluster=~"$cluster", namespace=~"$namespace"}[$__rate_interval]))
          |||,
        ),
      ])
      + stackedPanelMixin
    ),
  ],

  local lokiWritePanels(y_offset) = [
    panel.newRow(title='loki.write', y=y_offset),

    // Loki write success rate
    (
      panel.new(title='Write requests success rate in $cluster', type='timeseries') +
      panel.withUnit('%') +
      panel.withMax(100) +
      panel.withDescription(|||
        Percentage of logs sent by loki.write that succeeded.
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by($groupBy) (rate(loki_write_request_duration_seconds_bucket{status_code=~"2..", cluster=~"$cluster",namespace=~"$namespace"}[$__rate_interval]))
            /
            sum by($groupBy) (rate(loki_write_request_duration_seconds_bucket{cluster=~"$cluster",namespace=~"$namespace"}[$__rate_interval])) * 100 
          |||,
        ),
      ])
    ),

    // Loki write latency
    (
      panel.new(title='Write latency in $cluster', type='timeseries') +
      panel.withDescription(|||
        Bytes dropped per second.
      |||) +
      panel.withUnit('s') +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            histogram_quantile(
              0.99,
              sum by (le, $groupBy) (
            	rate(loki_write_request_duration_seconds_bucket{cluster=~"$cluster", namespace=~"$namespace"}[$__rate_interval])
              )
            )
          |||,
          legendFormat='{{pod}} p99'
        ),
        panel.newQuery(
          expr=|||
            histogram_quantile(
              0.95,
              sum by (le, $groupBy) (
            	rate(loki_write_request_duration_seconds_bucket{cluster=~"$cluster", namespace=~"$namespace"}[$__rate_interval])
              )
            )
          |||,
          legendFormat='{{pod}} p95'
        ),
        panel.newQuery(
          expr=|||
            histogram_quantile(
              0.50,
              sum by (le, $groupBy) (
            	rate(loki_write_request_duration_seconds_bucket{cluster=~"$cluster", namespace=~"$namespace"}[$__rate_interval])
              )
            )
          |||,
          legendFormat='{{pod}} p50'
        ),
      ])
    ),

    // Loki write entries sent
    (
      panel.new(title='Entries sent in $cluster', type='timeseries') +
      panel.withDescription(|||
        Number of loki entries sent per second.
      |||) +
      panel.withUnit('cps') +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by ($groupBy) (rate(loki_write_sent_entries_total{cluster=~"$cluster", namespace=~"$namespace"}[$__rate_interval]))
          |||,
        ),
      ]) + stackedPanelMixin
    ),

    // Loki write entries dropped
    (
      panel.new(title='Entries dropped in $cluster', type='timeseries') +
      panel.withDescription(|||
        Number of loki entries dropped.
      |||) +
      panel.withUnit('cps') +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by($groupBy) (rate(loki_write_dropped_entries_total{cluster=~"$cluster",namespace="$namespace"}[$__rate_interval]))
          |||,
        ),
      ]) + stackedPanelMixin
    ),

    // Loki write bytes sent
    (
      panel.new(title='Bytes sent in $cluster', type='timeseries') +
      panel.withDescription(|||
        Bytes sent per second.
      |||) +
      panel.withUnit('Bps') +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by ($groupBy) (rate(loki_write_sent_bytes_total{cluster=~"$cluster", namespace=~"$namespace"}[$__rate_interval]))
          |||,
        ),
      ]) + stackedPanelMixin
    ),

    // Loki write bytes dropped
    (
      panel.new(title='Bytes dropped in $cluster', type='timeseries') +
      panel.withDescription(|||
        Bytes dropped per second.
      |||) +
      panel.withUnit('Bps') +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by($groupBy) (rate(loki_write_dropped_bytes_total{cluster=~"$cluster",namespace="$namespace"}[$__rate_interval]))
          |||,
        ),
      ]) + stackedPanelMixin
    ),
  ],

  local panels =
    sourceFilePanels(y_offset=0) +
    journalFilePanels(y_offset=11) +
    lokiWritePanels(y_offset=22),

  local lokiTemplateVariables = [
    dashboard.newTemplateVariableCustom('groupBy', 'namespace,pod'),
  ],

  local templateVariables =
    templates.newTemplateVariablesList(
      filterSelector=$._config.filterSelector,
      enableK8sCluster=$._config.enableK8sCluster,
      includeInstance=true,
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels
    )
    + lokiTemplateVariables,

  [filename]:
    dashboard.new(name='Alloy / Loki Components', tag=$._config.dashboardTag) +
    dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin(templateVariables) +
	// TODO: what is this?
    dashboard.withAnnotations([
      dashboard.newLokiAnnotation('Deployments', '{cluster=~"$cluster", container="kube-diff-logger"} | json | namespace_extracted="alloy" | name_extracted=~"alloy.*"', 'rgba(0, 211, 255, 1)'),
    ]) +
    dashboard.withPanelsMixin(
      panels
    ),
}
