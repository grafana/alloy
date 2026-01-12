local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local templates = import './utils/templates.libsonnet';
local filename = 'alloy-loki.json';

{

  local sourceFilePanels(y_offset) = [
    panel.newRow(title='loki.source.file', y=y_offset),

    // Active files being tailed
    (
      panel.new(title='Active files count $cluster', type='timeseries') +
      panel.withStacked() +
      panel.withUnit('files') +
      panel.withDescription(|||
        Active files being tailed.
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by(instance) (loki_source_file_files_active_total{%(instanceSelector)s})
          ||| % $._config,
        ),
      ])
    ),

    // Lines read per second
    (
      panel.new(title='Lines read in $cluster', type='timeseries') +
      panel.withStacked() +
      panel.withUnit('cps') +
      panel.withDescription(|||
        Successful file reads.
      |||) +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by (instance) (rate(loki_source_file_read_lines_total{%(instanceSelector)s}[$__rate_interval]))
          ||| % $._config,
        ),
      ])
    ),
  ],

  local journalFilePanels(y_offset) = [
    panel.newRow(title='loki.source.journal', y=y_offset),

    // Journal lines
    (
      panel.new(title='Journal lines read in $cluster', type='timeseries') +
      panel.withStacked() +
      panel.withUnit('cps') +
      panel.withDescription(|||
        Successful journal reads.	
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by (instance) (rate(loki_source_journal_target_lines_total{%(instanceSelector)s}[$__rate_interval]))
          ||| % $._config,
        ),
      ])
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
            sum by(instance) (rate(loki_write_request_duration_seconds_bucket{%(instanceSelector)s, status_code=~"2..", host=~"$url"}[$__rate_interval]))
            /
            sum by(instance) (rate(loki_write_request_duration_seconds_bucket{%(instanceSelector)s, host=~"$url"}[$__rate_interval])) * 100 
          ||| % $._config,
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
              sum by (le, instance) (
            	rate(loki_write_request_duration_seconds_bucket{%(instanceSelector)s, host=~"$url"}[$__rate_interval])
              )
            )
          ||| % $._config,
          legendFormat='{{instance}} p99'
        ),
        panel.newQuery(
          expr=|||
            histogram_quantile(
              0.95,
              sum by (le, instance) (
            	rate(loki_write_request_duration_seconds_bucket{%(instanceSelector)s, host=~"$url"}[$__rate_interval])
              )
            )
          ||| % $._config,
          legendFormat='{{instance}} p95'
        ),
        panel.newQuery(
          expr=|||
            histogram_quantile(
              0.50,
              sum by (le, instance) (
            	rate(loki_write_request_duration_seconds_bucket{%(instanceSelector)s, host=~"$url"}[$__rate_interval])
              )
            )
          ||| % $._config,
          legendFormat='{{instance}} p50'
        ),
      ])
    ),

    // Loki write entries sent
    (
      panel.new(title='Entries sent in $cluster', type='timeseries') +
      panel.withDescription(|||
        Entries sent per second.
      |||) +
      panel.withStacked() +
      panel.withUnit('cps') +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by (instance) (rate(loki_write_sent_entires_total{%(instanceSelector)s, host=~"$url"}[$__rate_interval]))
          ||| % $._config,
        ),
      ])
    ),

    // Loki write entries dropped
    (
      panel.new(title='Entries dropped in $cluster', type='timeseries') +
      panel.withDescription(|||
        Entries dropped per second.
      |||) +
      panel.withStacked() +
      panel.withUnit('cps') +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by(instance) (rate(loki_write_dropped_entries_total{%(instanceSelector)s, host=~"$url"}[$__rate_interval]))
          ||| % $._config,
        ),
      ])
    ),

    // Loki write bytes sent
    (
      panel.new(title='Bytes sent in $cluster', type='timeseries') +
      panel.withDescription(|||
        Bytes sent per second.
      |||) +
      panel.withStacked() +
      panel.withUnit('Bps') +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by (instance) (rate(loki_write_sent_bytes_total{%(instanceSelector)s, host=~"$url"}[$__rate_interval]))
          ||| % $._config,
        ),
      ])
    ),

    // Loki write bytes dropped
    (
      panel.new(title='Bytes dropped in $cluster', type='timeseries') +
      panel.withDescription(|||
        Bytes dropped per second.
      |||) +
      panel.withStacked() +
      panel.withUnit('Bps') +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum by(instance) (rate(loki_write_dropped_bytes_total{%(instanceSelector)s, host=~"$url"}[$__rate_interval]))
          ||| % $._config,
        ),
      ])
    ),
  ],

  local panels =
    sourceFilePanels(y_offset=0) +
    journalFilePanels(y_offset=11) +
    lokiWritePanels(y_offset=22),

  local k8sEndpointQuery =
    if std.isEmpty($._config.filterSelector) then
      |||
        label_values(loki_write_sent_bytes_total{cluster=~"$cluster", namespace=~"$namespace", job="$job", instance=~"$instance"}, host)
      |||
    else
      |||
        label_values(loki_write_sent_bytes_total{%(filterSelector)s, cluster=~"$cluster", namespace=~"$namespace", job="$job", instance=~"$instance"}, host)
      ||| % $._config,

  local endpointQuery =
    if std.isEmpty($._config.filterSelector) then
      |||
        label_values(loki_write_sent_bytes_total{job="$job", instance=~"$instance"}, host)
      |||
    else
      |||
        label_values(loki_write_sent_bytes_total{%(filterSelector)s, job="$job", instance=~"$instance"}, host)
      ||| % $._config,

  local lokiTemplateVariables =
    if $._config.enableK8sCluster then
      [
        dashboard.newMultiTemplateVariable(
          name='url',
          query=k8sEndpointQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels
        ),
      ]
    else
      [
        dashboard.newMultiTemplateVariable(
          name='url',
          query=endpointQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels
        ),
      ],

  local templateVariables =
    templates.newTemplateVariablesList(
      filterSelector=$._config.filterSelector,
      enableK8sCluster=$._config.enableK8sCluster,
      includeInstance=true,
      setenceCaseLabels=$._config.useSetenceCaseTemplateLabels
    ) + lokiTemplateVariables,

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
