local dashboard = import './utils/dashboard.jsonnet';
local panel = import './utils/panel.jsonnet';
local templates = import './utils/templates.libsonnet';
local filename = 'alloy-prometheus-remote-write.json';

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

  local scrapePanels(y_offset) = [
    panel.newRow(title='prometheus.scrape', y=y_offset),

    // Scrape success rate
    (
      panel.new(title='Scrape success rate in $cluster', type='timeseries') +
      panel.withUnit('percentunit') +
      panel.withDescription(|||
        Percentage of targets successfully scraped by prometheus.scrape
        components.

        This metric is calculated by dividing the number of targets
        successfully scraped by the total number of targets scraped,
        across all the namespaces in the selected cluster.

        Low success rates can indicate a problem with scrape targets,
        stale service discovery, or Alloy misconfiguration.
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            sum(up{job=~"$job", cluster=~"$cluster"})
            /
            count (up{job=~"$job", cluster=~"$cluster"})
          |||,
          legendFormat='% of targets successfully scraped',
        ),
      ])
    ),

    // Scrape duration
    (
      panel.new(title='Scrape duration in $cluster', type='timeseries') +
      panel.withUnit('s') +
      panel.withDescription(|||
        Duration of successful scrapes by prometheus.scrape components,
        across all the namespaces in the selected cluster.

        This metric should be below your configured scrape interval.
        High durations can indicate a problem with a scrape target or
        a performance issue with Alloy.
      |||) +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 12, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr=|||
            quantile(0.99, scrape_duration_seconds{job=~"$job", cluster=~"$cluster"})
          |||,
          legendFormat='p99',
        ),
        panel.newQuery(
          expr=|||
            quantile(0.95, scrape_duration_seconds{job=~"$job", cluster=~"$cluster"})
          |||,
          legendFormat='p95',
        ),
        panel.newQuery(
          expr=|||
            quantile(0.50, scrape_duration_seconds{job=~"$job", cluster=~"$cluster"})
          |||,
          legendFormat='p50',
        ),

      ])
    ),
  ],

  local remoteWritePanels(y_offset) = [
    panel.newRow(title='prometheus.remote_write', y=y_offset),

    // WAL delay
    (
      panel.new(title='WAL delay', type='timeseries') +
      panel.withUnit('s') +
      panel.withDescription(|||
        How far behind prometheus.remote_write from samples recently written
        to the WAL.

        Each endpoint prometheus.remote_write is configured to send metrics
        has its own delay. The time shown here is the sum across all
        endpoints for the given component.

        It is normal for the WAL delay to be within 1-3 scrape intervals. If
        the WAL delay continues to increase beyond that amount, try
        increasing the number of maximum shards.
      |||) +
      panel.withPosition({ x: 0, y: 1 + y_offset, w: 6, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum by (instance, component_path, component_id) (
              prometheus_remote_storage_highest_timestamp_in_seconds{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component"}
              - ignoring(url, remote_name) group_right(instance)
              prometheus_remote_storage_queue_highest_sent_timestamp_seconds{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}
            )
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
      ])
    ),

    // Data write throughput
    (
      panel.new(title='Data write throughput', type='timeseries') +
      stackedPanelMixin +
      panel.withUnit('Bps') +
      panel.withDescription(|||
        Rate of data containing samples and metadata sent by
        prometheus.remote_write.
      |||) +
      panel.withPosition({ x: 6, y: 1 + y_offset, w: 6, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum without (remote_name, url) (
                rate(prometheus_remote_storage_bytes_total{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval]) +
                rate(prometheus_remote_storage_metadata_bytes_total{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval])
            )
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
      ])
    ),

    // Write latency
    (
      panel.new(title='Write latency', type='timeseries') +
      panel.withUnit('s') +
      panel.withDescription(|||
        Latency of writes to the remote system made by
        prometheus.remote_write.
      |||) +
      panel.withPosition({ x: 12, y: 1 + y_offset, w: 6, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            histogram_quantile(0.99, sum by (le) (
              rate(prometheus_remote_storage_sent_batch_duration_seconds_bucket{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval])
            ))
          ||| % $._config,
          legendFormat='99th percentile',
        ),
        panel.newQuery(
          expr= |||
            histogram_quantile(0.50, sum by (le) (
              rate(prometheus_remote_storage_sent_batch_duration_seconds_bucket{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval])
            ))
          ||| % $._config,
          legendFormat='50th percentile',
        ),
        panel.newQuery(
          expr= |||
            sum(rate(prometheus_remote_storage_sent_batch_duration_seconds_sum{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component"}[$__rate_interval])) /
            sum(rate(prometheus_remote_storage_sent_batch_duration_seconds_count{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component"}[$__rate_interval]))
          ||| % $._config,
          legendFormat='Average',
        ),
      ])
    ),

    // Shards
    (
      local minMaxOverride = {
        properties: [{
          id: 'custom.lineStyle',
          value: {
            dash: [10, 15],
            fill: 'dash',
          },
        }, {
          id: 'custom.showPoints',
          value: 'never',
        }, {
          id: 'custom.hideFrom',
          value: {
            legend: true,
            tooltip: false,
            viz: false,
          },
        }],
      };

      panel.new(title='Shards', type='timeseries') {
        fieldConfig+: {
          overrides: [
            minMaxOverride { matcher: { id: 'byName', options: 'Minimum' } },
            minMaxOverride { matcher: { id: 'byName', options: 'Maximum' } },
          ],
        },
      } +
      panel.withUnit('none') +
      panel.withDescription(|||
        Total number of shards which are concurrently sending samples read
        from the Write-Ahead Log.

        Shards are bound to a minimum and maximum, displayed on the graph.
        The lowest minimum and the highest maximum across all clients is
        shown.

        Each client has its own set of shards, minimum shards, and maximum
        shards; filter to a specific URL to display more granular
        information.
      |||) +
      panel.withPosition({ x: 18, y: 1 + y_offset, w: 6, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum without (remote_name, url) (
                prometheus_remote_storage_shards{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}
            )
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
        panel.newQuery(
          expr= |||
            min (
                prometheus_remote_storage_shards_min{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}
            )
          ||| % $._config,
          legendFormat='Minimum',
        ),
        panel.newQuery(
          expr= |||
            max (
                prometheus_remote_storage_shards_max{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}
            )
          ||| % $._config,
          legendFormat='Maximum',
        ),
      ])
    ),

    // Sent samples / second
    (
      panel.new(title='Sent samples / second', type='timeseries') +
      stackedPanelMixin +
      panel.withUnit('cps') +
      panel.withDescription(|||
        Total outgoing samples sent by prometheus.remote_write.
      |||) +
      panel.withPosition({ x: 0, y: 11 + y_offset, w: 8, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum without (url, remote_name) (
              rate(prometheus_remote_storage_samples_total{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval])
            )
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
      ])
    ),

    // Failed samples / second
    (
      panel.new(title='Failed samples / second', type='timeseries') +
      stackedPanelMixin +
      panel.withUnit('cps') +
      panel.withDescription(|||
        Rate of samples which prometheus.remote_write could not send due to
        non-recoverable errors.
      |||) +
      panel.withPosition({ x: 8, y: 11 + y_offset, w: 8, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum without (url,remote_name) (
              rate(prometheus_remote_storage_samples_failed_total{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval])
            )
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
      ])
    ),

    // Retried samples / second
    (
      panel.new(title='Retried samples / second', type='timeseries') +
      stackedPanelMixin +
      panel.withUnit('cps') +
      panel.withDescription(|||
        Rate of samples which prometheus.remote_write attempted to resend
        after receiving a recoverable error.
      |||) +
      panel.withPosition({ x: 16, y: 11 + y_offset, w: 8, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum without (url,remote_name) (
              rate(prometheus_remote_storage_samples_retried_total{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"}[$__rate_interval])
            )
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
      ])
    ),

    // Active series (Total)
    (
      panel.new(title='Active series (total)', type='timeseries') {
        options+: {
          legend+: {
            showLegend: false,
          },
        },
      } +
      panel.withUnit('short') +
      panel.withDescription(|||
        Total number of active series across all components.

        An "active series" is a series that prometheus.remote_write recently
        received a sample for. Active series are garbage collected whenever a
        truncation of the WAL occurs.
      |||) +
      panel.withPosition({ x: 0, y: 21 + y_offset, w: 8, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum(prometheus_remote_write_wal_storage_active_series{%(instanceSelector)s, component_path=~"$component_path", component_id=~"$component", url=~"$url"})
          ||| % $._config,
          legendFormat='Series',
        ),
      ])
    ),

    // Active series (by instance/component)
    (
      panel.new(title='Active series (by instance/component)', type='timeseries') +
      panel.withUnit('short') +
      panel.withDescription(|||
        Total number of active series which are currently being tracked by
        prometheus.remote_write components, with separate lines for each Alloy instance.

        An "active series" is a series that prometheus.remote_write recently
        received a sample for. Active series are garbage collected whenever a
        truncation of the WAL occurs.
      |||) +
      panel.withPosition({ x: 8, y: 21 + y_offset, w: 8, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            prometheus_remote_write_wal_storage_active_series{%(instanceSelector)s, component_id!="", component_path=~"$component_path", component_id=~"$component", url=~"$url"}
          ||| % $._config,
          legendFormat='{{instance}} / {{component_path}} {{component_id}}',
        ),
      ])
    ),

    // Active series (by component)
    (
      panel.new(title='Active series (by component)', type='timeseries') +
      panel.withUnit('short') +
      panel.withDescription(|||
        Total number of active series which are currently being tracked by
        prometheus.remote_write components, aggregated across all instances.

        An "active series" is a series that prometheus.remote_write recently
        received a sample for. Active series are garbage collected whenever a
        truncation of the WAL occurs.
      |||) +
      panel.withPosition({ x: 16, y: 21 + y_offset, w: 8, h: 10 }) +
      panel.withQueries([
        panel.newQuery(
          expr= |||
            sum by (component_path, component_id) (prometheus_remote_write_wal_storage_active_series{%(instanceSelector)s, component_id!="", component_path=~"$component_path", component_id=~"$component", url=~"$url"})
          ||| % $._config,
          legendFormat='{{component_path}} {{component_id}}',
        ),
      ])
    ),
  ],

  local panels = 
    if $._config.enableK8sCluster then
      // First row, offset is 0
      scrapePanels(y_offset=0) +
      // Scrape panels take 11 units, so offset next row by 11.
      remoteWritePanels(y_offset=11)
    else
      remoteWritePanels(y_offset=0),

  local k8sComponentPathQuery = 
    if std.isEmpty($._config.filterSelector) then
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{cluster=~"$cluster", namespace=~"$namespace", job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*", component_path=~".*"}, component_path)
    |||
    else
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{%(filterSelector)s, cluster=~"$cluster", namespace=~"$namespace", job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*", component_path=~".*"}, component_path)
    ||| % $._config,

  local k8sComponentQuery = 
    if std.isEmpty($._config.filterSelector) then
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{cluster=~"$cluster", namespace=~"$namespace", job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*"}, component_id)
    |||
    else
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{%(filterSelector)s, cluster=~"$cluster", namespace=~"$namespace", job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*"}, component_id)
    ||| % $._config,

  local k8sUrlQuery = 
    if std.isEmpty($._config.filterSelector) then
    |||
        label_values(prometheus_remote_storage_sent_batch_duration_seconds_sum{cluster=~"$cluster", namespace=~"$namespace", job="$job", instance=~"$instance", component_id=~"$component"}, url)
    |||
    else
    |||
        label_values(prometheus_remote_storage_sent_batch_duration_seconds_sum{%(filterSelector)s, cluster=~"$cluster", namespace=~"$namespace", job="$job", instance=~"$instance", component_id=~"$component"}, url)
    ||| % $._config,
  
  local componentPathQuery = 
    if std.isEmpty($._config.filterSelector) then
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*", component_path=~".*"}, component_path)
    |||
    else
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{%(filterSelector)s, job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*", component_path=~".*"}, component_path)
    ||| % $._config,

  local componentQuery = 
    if std.isEmpty($._config.filterSelector) then
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*"}, component_id)
    |||
    else
    |||
        label_values(prometheus_remote_write_wal_samples_appended_total{%(filterSelector)s, job=~"$job", instance=~"$instance", component_id=~"prometheus.remote_write.*"}, component_id)
    ||| % $._config,

  local urlQuery = 
    if std.isEmpty($._config.filterSelector) then
    |||
        label_values(prometheus_remote_storage_sent_batch_duration_seconds_sum{job="$job", instance=~"$instance", component_id=~"$component"}, url)
    |||
    else
    |||
        label_values(prometheus_remote_storage_sent_batch_duration_seconds_sum{%(filterSelector)s, job="$job", instance=~"$instance", component_id=~"$component"}, url)
    ||| % $._config,

  local prometheusTemplateVariables =
    if $._config.enableK8sCluster then
      [        
        dashboard.newMultiTemplateVariable(
          name='component_path', 
          query=k8sComponentPathQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),
        dashboard.newMultiTemplateVariable(
          name='component', 
          query=k8sComponentQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),          
        dashboard.newMultiTemplateVariable(
          name='url', 
          query= k8sUrlQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),
      ]
    else
      [       
        dashboard.newMultiTemplateVariable(
          name='component_path', 
          query=componentPathQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),
        dashboard.newMultiTemplateVariable(
          name='component', 
          query=componentQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),          
        dashboard.newMultiTemplateVariable(
          name='url', 
          query=urlQuery,
          setenceCaseLabels=$._config.useSetenceCaseTemplateLabels),
      ],
    
    local templateVariables = 
      templates.newTemplateVariablesList(
        filterSelector=$._config.filterSelector, 
        enableK8sCluster=$._config.enableK8sCluster, 
        includeInstance=true,
        setenceCaseLabels=$._config.useSetenceCaseTemplateLabels)
      + prometheusTemplateVariables,

  [filename]:
    dashboard.new(name='Alloy / Prometheus Components', tag=$._config.dashboardTag) +
    dashboard.withDocsLink(
      url='https://grafana.com/docs/alloy/latest/reference/components/prometheus.remote_write/',
      desc='Component documentation',
    ) +
    dashboard.withDashboardsLink(tag=$._config.dashboardTag) +
    dashboard.withUID(std.md5(filename)) +
    dashboard.withTemplateVariablesMixin(templateVariables) +
    // TODO(@tpaschalis) Make the annotation optional.
    dashboard.withAnnotations([
      dashboard.newLokiAnnotation('Deployments', '{cluster=~"$cluster", container="kube-diff-logger"} | json | namespace_extracted="alloy" | name_extracted=~"alloy.*"', 'rgba(0, 211, 255, 1)'),
    ]) +
    dashboard.withPanelsMixin(
      panels
    ),
}
