# Changelog

## [1.12.2](https://github.com/grafana/alloy/compare/v1.12.1...v1.12.2) (2026-01-08)


### Bug Fixes 🐛

* Add missing configuration parameter `deployment_name_from_replicaset` to k8sattributes processor ([5b90a9d](https://github.com/grafana/alloy/commit/5b90a9d391d222eb9c8ea1e40e38a9dbbbd06ffd))
* **database_observability:** Fix schema_details collector to fetch column definitions with case sensitive table names ([#4872](https://github.com/grafana/alloy/issues/4872)) ([560dff4](https://github.com/grafana/alloy/commit/560dff4ccef090e2db85ef6dd9e59aeacf54e3f2))
* **deps:** Update jose2go to 1.7.0 ([#4858](https://github.com/grafana/alloy/issues/4858)) ([dfdd341](https://github.com/grafana/alloy/commit/dfdd341c8da5e7b972905d166a497e3093323be2))
* **deps:** Update npm dependencies [backport] ([#5201](https://github.com/grafana/alloy/issues/5201)) ([8e06c26](https://github.com/grafana/alloy/commit/8e06c2673c0f5790eba84e9f7091270b3ab0bf2d))
* Ensure the squid exporter wrapper properly brackets ipv6 addresses [backport] ([#5205](https://github.com/grafana/alloy/issues/5205)) ([e329cc6](https://github.com/grafana/alloy/commit/e329cc6ebdfd7fb52034b5f215082e2fac9640f6))
* Preserve meta labels in loki.source.podlogs ([#5097](https://github.com/grafana/alloy/issues/5097)) ([ab4b21e](https://github.com/grafana/alloy/commit/ab4b21ec0c8b4e892ffa39035c6a53149ee05555))
* Prevent panic in import.git when update fails [backport] ([#5204](https://github.com/grafana/alloy/issues/5204)) ([c82fbae](https://github.com/grafana/alloy/commit/c82fbae5431dca9fe3ba071c99978babc2f9b5b1))
* show correct fallback alloy version instead of v1.13.0 ([#5110](https://github.com/grafana/alloy/issues/5110)) ([b72be99](https://github.com/grafana/alloy/commit/b72be995908ac761c0ea9a4f881367dc6ec6da13))

## [1.12.1](https://github.com/grafana/alloy/compare/v1.12.0...v1.12.1) (2025-12-15)


### Bug Fixes 🐛

* update to Beyla 2.7.10 ([#5019](https://github.com/grafana/alloy/issues/5019)) ([c149393](https://github.com/grafana/alloy/commit/c149393881e8c155681de9c03f8701b1fdbc6ea4))

## [1.12.0](https://github.com/grafana/alloy/compare/v1.11.3...v1.12.0) (2025-12-01)

### Breaking changes

- `prometheus.exporter.blackbox`, `prometheus.exporter.snmp` and `prometheus.exporter.statsd` now use the component ID instead of the hostname as
  their `instance` label in their exported metrics. This is a consequence of a bug fix that could lead to missing data when using the exporter
  with clustering. If you would like to retain the previous behaviour, you can use `discovery.relabel` with `action = "replace"` rule to
  set the `instance` label to `sys.env("HOSTNAME")`. (@thampiotr)

### Features

- Add `otelcol.exporter.file` component to write metrics, logs, and traces to disk with optional rotation, compression, and grouping by resource attribute. (@madhub)

- (_Experimental_) Add an `otelcol.receiver.cloudflare` component to receive
  logs pushed by Cloudflare's [LogPush](https://developers.cloudflare.com/logs/logpush/) jobs. (@x1unix)

- (_Experimental_) Additions to experimental `database_observability.mysql` component:
  - `explain_plans`
    - collector now changes schema before returning the connection to the pool (@cristiangreco)
    - collector now passes queries more permissively, expressly to allow queries beginning in `with` (@rgeyer)
  - enable `explain_plans` collector by default (@rgeyer)

- (_Experimental_) Additions to experimental `database_observability.postgres` component:
  - `explain_plans`
    - added the explain plan collector (@rgeyer)
    - collector now passes queries more permissively, expressly to allow queries beginning in `with` (@rgeyer)
  - `query_samples`
    - add `user` field to wait events within `query_samples` collector (@gaantunes)
    - rework the query samples collector to buffer per-query execution state across scrapes and emit finalized entries (@gaantunes)
    - process turned idle rows to calculate finalization times precisely and emit first seen idle rows (@gaantunes)
  - `query_details`
    - escape queries coming from pg_stat_statements with quotes (@gaantunes)
  - enable `explain_plans` collector by default (@rgeyer)
  - safely generate server_id when UDP socket used for database connection (@matthewnolf)
  - add table registry and include "validated" in parsed table name logs (@fridgepoet)
  - add database exclusion list for Postgres schema_details collector (@fridgepoet)

- Add `otelcol.exporter.googlecloudpubsub` community component to export metrics, traces, and logs to Google Cloud Pub/Sub topic. (@eraac)

- Add `structured_metadata_drop` stage for `loki.process` to filter structured metadata. (@baurmatt)

- Send remote config status to the remote server for the remotecfg service. (@erikbaranowski)

- Send effective config to the remote server for the remotecfg service. (@erikbaranowski)

- Add a `stat_statements` configuration block to the `prometheus.exporter.postgres` component to enable selecting both the query ID and the full SQL statement. The new block includes one option to enable statement selection, and another to configure the maximum length of the statement text. (@SimonSerrano)

- Add `truncate` stage for `loki.process` to truncate log entries, label values, and structured_metadata values. (@dehaansa)

- Add `u_probe_links` & `load_probe` configuration fields to alloy pyroscope.ebpf to extend configuration of the opentelemetry-ebpf-profiler to allow uprobe profiling and dynamic probing. (@luweglarz)

- Add `verbose_mode` configuration fields to alloy pyroscope.ebpf to be enable ebpf-profiler verbose mode. (@luweglarz)

- Add `file_match` block to `loki.source.file` for built-in file discovery using glob patterns. (@kalleep)

- Add a `regex` argument to the `structured_metadata` stage in `loki.process` to extract labels matching a regular expression. (@timonegk)

- Add `lazy_mode` argument to the `pyroscope.ebpf` to defer eBPF profiler startup until there are targets to profile. (@luweglarz)

- OpenTelemetry Collector dependencies upgraded from v0.134.0 to v0.139.0. (@dehaansa)
  - All `otelcol.receiver.*` components leveraging an HTTP server can configure HTTP keep alive behavior with `keep_alives_enabled`.
  - All `otelcol.exporter.*` components providing the `sending_queue` > `batch` block have default `batch` values.
  - The `otelcol.processor.k8sattributes` component has support for extracting annotations from k8s jobs and daemonsets.
  - The `otelcol.processor.resourcedecetion` component supports nine new detectors.
  - The `otelcol.exporter.kafka` component supports partitioning logs by trace ID (`partition_logs_by_trace_id`) and configuring default behavior if topic does not exist (`allow_auto_topic_creation`).
  - The `otelcol.receiver.kafka` component has new configuration options `max_partition_fetch_size`, `rack_id`, and `use_leader_epoch`.
  - The `otelcol.exporter.s3` component has new configuration options `s3_base_prefix` and `s3_partition_timezone`.
  - The `otelcol.processor.servicegraph` component now supports defining the maximum number of buckets for generated exponential histograms.
  - See the upstream [core][https://github.com/open-telemetry/opentelemetry-collector/blob/v0.139.0/CHANGELOG.md] and [contrib][https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.139.0/CHANGELOG.md] changelogs for more details.

- A new `mimir.alerts.kubernetes` component which discovers `AlertmanagerConfig` Kubernetes resources and loads them into a Mimir instance. (@ptodev)

- Mark `stage.windowsevent` block in the `loki.process` component as GA. (@kgeckhart)

### Enhancements

- Add per-application rate limiting with the `strategy` attribute in the `faro.receiver` component, to prevent one application from consuming the rate limit quota of others. (@hhertout)

- Add support of `tls` in components `loki.source.(awsfirehose|gcplog|heroku|api)` and `prometheus.receive_http` and `pyroscope.receive_http`. (@fgouteroux)

- Remove SendSIGKILL=no from unit files and recommendations (@oleg-kozlyuk-grafana)

- Reduce memory overhead of `prometheus.remote_write`'s WAL by lowering the size of the allocated series storage. (@kgeckhart)

- Reduce lock wait/contention on the labelstore.LabelStore by removing unecessary usage from `prometheus.relabel`. (@kgeckhart)

- `prometheus.exporter.postgres` dependency has been updated to v0.18.1. This includes new `stat_progress_vacuum` and `buffercache_summary` collectors, as well as other bugfixes and enhancements. (@cristiangreco)

- Update Beyla component to 2.7.8. (@grcevski)

- Support delimiters in `stage.luhn`. (@dehaansa)

- pyroscope.java: update async-profiler to 4.2 (@korniltsev-grafanista)
- Improve debug info output from exported receivers (loki, prometheus and pyroscope). (@kalleep)

- `prometheus.exporter.unix`: Add an `arp` config block to configure the ARP collector. (@ptodev)

- `prometheus.exporter.snowflake` dependency has been updated to 20251016132346-6d442402afb2, which updates data ownership queries to use `last_over_time` for a 24 hour period. (@dasomeone)

- `loki.source.podlogs` now supports `preserve_discovered_labels` parameter to preserve discovered pod metadata labels for use by downstream components. (@QuentinBisson)

- Rework underlying framework of Alloy UI to use Vite instead of Create React App. (@jharvey10)

- Use POST requests for remote config requests to avoid hitting http2 header limits. (@tpaschalis)

- `loki.source.api` during component shutdown will now reject all the inflight requests with status code 503 after `graceful_shutdown_timeout` has expired. (@kalleep)

- `kubernetes.discovery` Add support for attaching namespace metadata. (@kgeckhart)

- Add `meta_cache_address` to `beyla.ebpf` component. (@skl)

### Bugfixes

- Stop `loki.source.kubernetes` discarding log lines with duplicate timestamps. (@ciaranj)

- Fix direction of arrows for pyroscope components in UI graph. (@dehaansa)

- Only log EOF errors for syslog port investigations in `loki.source.syslog` as Debug, not Warn. (@dehaansa)

- Fix prometheus.exporter.process ignoring the `remove_empty_groups` argument. (@mhamzahkhan)

- Fix issues with "unknown series ref when trying to add exemplar" from `prometheus.remote_write` by allowing series ref links to be updated if they change. (@kgeckhart)

- Fix `loki.source.podlogs` component to register the Kubernetes field index for `spec.nodeName` when node filtering is enabled, preventing "Index with name field:spec.nodeName does not exist" errors. (@QuentinBisson)

- Fix issue in `loki.source.file` where scheduling files could take too long. (@kalleep)

- Fix `loki.write` no longer includes internal labels `__`.  (@matt-gp)

- Fix missing native histograms custom buckets (NHCB) samples from `prometheus.remote_write`. (@krajorama)

- `otelcol.receiver.prometheus` now supports mixed histograms if `prometheus.scrape` has `honor_metadata` set to `true`. (@ptodev)
  A mixed histogram is one which has both classic and exponential buckets.

- `loki.source.file` has better support for non-UTF-8 encoded files. (@ptodev)
  * A BOM will be taken into account if the file is UTF-16 encoded and `encoding` is set to `UTF-16`. (Not `UTF-16BE` or `UTF-16LE`)
  * The carriage return symbol in Windows log files with CLRF endings will no longer be part of the log line.
  * These bugs used to cause some logs to show up with Chinese characters. Notably, this would happen on MSSQL UTF-16 LE logs.

- Fix the `loki.write` endpoint block's `enable_http2` attribute to actually affect the client. HTTP2 was previously disabled regardless of configuration. (@dehaansa)

- Optionally remove trailing newlines before appending entries in `stage.multiline`. (@dehaansa)

- `loki.source.api` no longer drops request when relabel rules drops a specific stream. (@kalleep)

v1.11.3
-----------------

### Enhancements

- Schedule new path targets faster in `loki.source.file`. (@kalleep)

- Add `prometheus.static.exporter` that exposes metrics specified in a text file in Prometheus exposition format. (@kalleep)

### Bugfixes

- `local.file_match` now publish targets faster whenever targets in arguments changes. (@kalleep)

- Fix `otelcol.exporter.splunkhec` arguments missing documented `otel_attrs_to_hec_metadata` block. (@dehaansa)

- Support Scrape Protocol specification in CRDS for `prometheus.operator.*` components. (@dehaansa)

- Fix panic in `otelcol.receiver.syslog` when no tcp block was configured. (@kalleep)

- Fix breaking changes in the texfile collector for `prometheus.exporter.windows`, and `prometheus.exporter.unix`, when prometheus/common was upgraded. (@kgeckhart)

- Support recovering from corrupted positions file entries in `loki.source.file`. (@dehaansa)

### Other changes

- Augment prometheus.scrape 'scheme' argument strengthening link to protocol. (@lewismc)

- Stop `faro.receiver` losing trace context when exception has stack trace. (@duartesaraiva98)

v1.11.2
-----------------

### Bugfixes

- Fix potential deadlock in `loki.source.journal` when stopping or reloading the component. (@thampiotr)

- Honor sync timeout when waiting for network availability for prometheus.operator.* components. (@dehaansa)

- Fix `prometheus.exporter.cloudwatch` to not always emit debug logs but respect debug property. (@kalleep)

- Fix an issue where component shutdown could block indefinitely by adding a warning log message and a deadline of 10 minutes. The deadline can be configured with the `--feature.component-shutdown-deadline` flag if the default is not suitable. (@thampiotr)

- Fix potential deadlocks in `loki.source.file` and `loki.source.journal` when component is shutting down. (@kalleep, @thampiotr)

v1.11.0
-----------------

### Breaking changes

- Prometheus dependency had a major version upgrade from v2.55.1 to v3.4.2. (@thampiotr)

  - The `.` pattern in regular expressions in PromQL matches newline characters now. With this change a regular expressions like `.*` matches strings that include `\n`. This applies to matchers in queries and relabel configs in Prometheus and Loki components.

  - The `enable_http2` in `prometheus.remote_write` component's endpoints has been changed to `false` by default. Previously, in Prometheus v2 the remote write http client would default to use http2. In order to parallelize multiple remote write queues across multiple sockets its preferable to not default to http2. If you prefer to use http2 for remote write you must now set `enable_http2` to `true` in your `prometheus.remote_write` endpoints configuration section.

  - The experimental CLI flag `--feature.prometheus.metric-validation-scheme` has been deprecated and has no effect. You can configure the metric validation scheme individually for each `prometheus.scrape` component.

  - Log message format has changed for some of the `prometheus.*` components as part of the upgrade to Prometheus v3.

  - The values of the `le` label of classic histograms and the `quantile` label of summaries are now normalized upon ingestion. In previous Alloy versions, that used Prometheus v2, the value of these labels depended on the scrape protocol (protobuf vs text format) in some situations. This led to label values changing based on the scrape protocol. E.g. a metric exposed as `my_classic_hist{le="1"}` would be ingested as `my_classic_hist{le="1"}` via the text format, but as `my_classic_hist{le="1.0"}` via protobuf. This changed the identity of the metric and caused problems when querying the metric. In current Alloy release, which uses Prometheus v3, these label values will always be normalized to a float like representation. I.e. the above example will always result in `my_classic_hist{le="1.0"}` being ingested into Prometheus, no matter via which protocol. The effect of this change is that alerts, recording rules and dashboards that directly reference label values as whole numbers such as `le="1"` will stop working.

    The recommended way to deal with this change is to fix references to integer `le` and `quantile` label values, but otherwise do nothing and accept that some queries that span the transition time will produce inaccurate or unexpected results.

  See the upstream [Prometheus v3 migration guide](https://prometheus.io/docs/prometheus/3.4/migration/) for more details.

- `prometheus.exporter.windows` dependency has been updated to v0.31.1. (@dehaansa)
  - There are various renamed metrics and two removed collectors (`cs`, `logon`), see the [v1.11 release notes][1_11-release-notes] for more information.

    [1_11-release-notes]: https://grafana.com/docs/alloy/latest/release-notes/#v111

- `scrape_native_histograms` attribute for `prometheus.scrape` is now set to `false`, whereas in previous versions of Alloy it would default to `true`. This means that it is no longer enough to just configure `scrape_protocols` to start with `PrometheusProto` to scrape native histograms - `scrape_native_histograms` has to be enabled. If `scrape_native_histograms` is enabled, `scrape_protocols` will automatically be configured correctly for you to include `PrometheusProto`. If you configure it explicitly, Alloy will validate that `PrometheusProto` is in the `scrape_protocols` list.

- Add `otel_attrs_to_hec_metadata` configuration block to `otelcol.exporter.splunkhec` to match `otelcol.receiver.splunkhec`. (@cgetzen)

- [`otelcol.processor.batch`] Two arguments have different default values. (@ptodev)
  - `send_batch_size` is now set to 2000 by default. It used to be 8192.
  - `send_batch_max_size` is now set to 3000 by default. It used to be 0.
  - This helps prevent issues with ingestion of batches that are too large.

- OpenTelemetry Collector dependencies upgraded from v0.128.0 to v0.134.0. (@ptodev)
  - The `otelcol.receiver.opencensus` component has been deprecated and will be removed in a future release, use `otelcol.receiver.otelp` instead.
  - [`otelcol.exporter.*`] The deprecated `blocking` argument in the `sending_queue` block has been removed.
    Use `block_on_overflow` instead.
  - [`otelcol.receiver.kafka`, `otelcol.exporter.kafka`]: Removed the `broker_addr` argument from the `aws_msk` block.
    Also removed the `SASL/AWS_MSK_IAM` authentication mechanism.
  - [`otelcol.exporter.splunkhec`] The `batcher` block is deprecated and will be removed in a future release. Use the `queue` block instead.
  - [`otelcol.exporter.loadbalancing`] Use a linear probe to decrease variance caused by hash collisions, which was causing a non-uniform distribution of loadbalancing.
  - [`otelcol.connector.servicegraph`] The `database_name_attribute` argument has been removed.
  - [`otelcol.connector.spanmetrics`] Adds a default maximum number of exemplars within the metric export interval.
  - [`otelcol.processor.tail_sampling`] Add a new `block_on_overflow` config attribute.

### Features

- Add the `otelcol.receiver.fluentforward` receiver to receive logs via Fluent Forward Protocol. (@rucciva)
- Add the `prometheus.enrich` component to enrich metrics using labels from `discovery.*` components. (@ArkovKonstantin)

- Add the `otelcol.receiver.awsecscontainermetrics` receiver (from upstream OTEL contrib) to read AWS ECS task- and container-level resource usage metrics. (@gregbrowndev)

- Add `node_filter` configuration block to `loki.source.podlogs` component to enable node-based filtering for pod discovery. When enabled, only pods running on the specified node will be discovered and monitored, significantly reducing API server load and network traffic in DaemonSet deployments. (@QuentinBisson)

- (_Experimental_) Additions to experimental `database_observability.mysql` component:
  - `query_sample` collector now supports auto-enabling the necessary `setup_consumers` settings (@cristiangreco)
  - `query_sample` collector is now compatible with mysql less than 8.0.28 (@cristiangreco)
  - include `server_id` label on log entries (@matthewnolf)
  - support receiving targets argument and relabel those to include `server_id` (@matthewnolf)
  - updated the config blocks and documentation (@cristiangreco)

- (_Experimental_) Additions to experimental `database_observability.postgres` component:
  - add `query_tables` collector for postgres (@matthewnolf)
  - add `cloud_provider.aws` configuration that enables optionally supplying the ARN of the database under observation. The ARN is appended to metric samples as labels for easier filtering and grouping of resources.
  - add `query_sample` collector for postgres (@gaantunes)
  - add `schema_details` collector for postgres (@fridgepoet)
  - include `server_id` label on logs and metrics (@matthewnolf)

- Add `otelcol.receiver.googlecloudpubsub` community component to receive metrics, traces, and logs from Google Cloud Pub/Sub subscription. (@eraac)

- Add otel collector converter for `otelcol.receiver.googlecloudpubsub`. (@kalleep)

- (_Experimental_) Add a `honor_metadata` configuration argument to the `prometheus.scrape` component.
  When set to `true`, it will propagate metric metadata to downstream components.

- Add a flag to pyroscope.ebpf alloy configuration to set the off-cpu profiling threshold. (@luweglarz)

- Add `encoding.url_encode` and `encoding.url_decode` std lib functions. (@kalleep)

### Enhancements

- Ensure text in the UI does not overflow node boundaries in the graph. (@blewis12)

- Fix `pyroscope.write` component's `AppendIngest` method to respect configured timeout and implement retry logic. The method now properly uses the configured `remote_timeout`, includes retry logic with exponential backoff, and tracks metrics for sent/dropped bytes and profiles consistently with the `Append` method. (@korniltsev)

- `pyroscope.write`, `pyroscope.receive_http` components include `trace_id` in logs and propagate it downstream. (@korniltsev)

- Improve logging in `pyroscope.write` component. (@korniltsev)

- Add comprehensive latency metrics to `pyroscope.write` component with endpoint-specific tracking for both push and ingest operations. (@korniltsev, @claude)

- `prometheus.scrape` now supports `convert_classic_histograms_to_nhcb`, `enable_compression`, `metric_name_validation_scheme`, `metric_name_escaping_scheme`, `native_histogram_bucket_limit`, and `native_histogram_min_bucket_factor` arguments. See reference documentation for more details. (@thampiotr)

- Add `max_send_message_size` configuration option to `loki.source.api` component to control the maximum size of requests to the push API. (@thampiotr)

- Add `protobuf_message` argument to `prometheus.remote_write` endpoint configuration to support both Prometheus Remote Write v1 and v2 protocols. The default remains `"prometheus.WriteRequest"` (v1) for backward compatibility. (@thampiotr)

- Update the `yet-another-cloudwatch-exporter` dependency to point to the prometheus-community repo as it has been donated. Adds a few new services to `prometheus.exporter.cloudwatch`. (@dehaansa, @BoweFlex, @andriikushch)

- `pyroscope.java` now supports configuring the `log_level` and `quiet` flags on async-profiler. (@deltamualpha)

- Add `application_host` and `network_inter_zone` features to `beyla.ebpf` component. (@marctc)

- Set the publisher name in the Windows installer to "Grafana Labs". (@martincostello)

- Switch to the community maintained fork of `go-jmespath` that has more features. (@dehaansa)

- Add a `stage.pattern` stage to `loki.process` that uses LogQL patterns to parse logs. (@dehaansa)

- Add support to validate references, stdlib functions and arguments when using validate command. (@kalleep)

- Update the `prometheus.exporter.process` component to get the `remove_empty_groups` option. (@dehaansa)

- Remove unnecessary allocations in `stage.static_labels`. (@kalleep)

- Upgrade `beyla.ebpf` from Beyla version v2.2.5 to v2.5.8 The full list of changes can be found in the [Beyla release notes](https://github.com/grafana/beyla/releases/tag/v2.5.2) (@marctc)

- `prometheus.exporter.azure` supports setting `interval` and `timespan` independently allowing for further look back when querying metrics. (@kgeckhart)

- `loki.source.journal` now supports `legacy_positon` block that can be used to translate Static Agent or Promtail position files. (@kalleep)

- Normalize attr key name in logfmt logger. (@zry98)

- (_Experimental_) Add an extra parameter to the `array.combine_maps` standard library function
  to enable preserving the first input list even if there is no match. (@ptodev)

- Reduce memory overhead of `prometheus.remote_write`'s WAL by bringing in an upstream change to only track series in a slice if there's a hash conflict. (@kgeckhart)

- Reduce log level from warning for `loki.write` when request fails and will be retried. (@kalleep)

- Fix slow updates to `loki.source.file` when only targets have changed and pipeline is blocked on writes. (@kalleep)

- Reduced allocation in `loki.write` when using external labels with mutliple endpoints. (@kalleep)

- The Windows installer and executables are now code signed. (@martincostello)

- Reduce compressed request size in `prometheus.write.queue` by ensuring append order is maintained when sending metrics to the WAL. (@kgeckhart)

- Add `protobuf_message` and `metadata_cache_size` arguments to `prometheus.write.queue` endpoint configuration to support both Prometheus Remote Write v1 and v2 protocols. The default remains `"prometheus.WriteRequest"` (v1) for backward compatibility. (@dehaansa)

- Reduce allocations for `loki.process` when `stage.template` is used. (@kalleep)

- Reduce CPU of `prometheus.write.queue` by eliminating duplicate calls to calculate the protobuf Size. (@kgeckhart)

- Use new cache for metadata cache in `prometheus.write.queue` and support disabling the metadata cache with it disable by default. (@kgeckhart, @dehaansa)

### Bugfixes

- Update `webdevops/go-common` dependency to resolve concurrent map write panic. (@dehaansa)

- Fix ebpf profiler metrics `pyroscope_ebpf_active_targets`, `pyroscope_ebpf_profiling_sessions_total`, `pyroscope_ebpf_profiling_sessions_failing_total` not being updated. (luweglarz)

- Fix `prometheus.operator.podmonitors` so it now handle portNumber from PodMonitor CRD. (@kalleep)

- Fix `pyroscope.receive_http` so it does not restart server if the server configuration has not changed. (@korniltsev)

- Increase default connection limit in `pyroscope.receive_http` from 100 to 16k. (@korniltsev)

- Fix issue in `prometheus.remote_write`'s WAL which could allow it to hold an active series forever. (@kgeckhart)

- Fix issue in static and promtail converter where metrics type was not properly handled. (@kalleep)

- Fix `prometheus.operator.*` components to allow them to scrape correctly Prometheus Operator CRDs. (@thomas-gouveia)

- Fix `database_observability.mysql` and `database_observability.postgres` crashing alloy process due to uncaught errors.

- Fix data race in`loki.source.docker` that could cause Alloy to panic. (@kalleep)

- Fix race conditions in `loki.source.syslog` where it could deadlock or cause port bind errors during config reload or shutdown. (@thampiotr)

- Fix `prometheus.exporter.redis` component so that it no longer ignores the `MaxDistinctKeyGroups` configuration option. If key group metrics are enabled, this will increase the cardinality of the generated metrics. (@stegosaurus21)

- **Fix `loki.source.podlogs` component to properly collect logs from Kubernetes Jobs and CronJobs.** Previously, the component would fail to scrape logs from short-lived or terminated jobs due to race conditions between job completion and pod discovery. The fix includes:
  - Job-aware termination logic with extended grace periods (10-60 seconds) to ensure all logs are captured
  - Proper handling of pod deletion and race conditions between job completion and controller cleanup
  - Separation of concerns: `shouldStopTailingContainer()` handles standard Kubernetes restart policies for regular pods, while `shouldStopTailingJobContainer()` handles job-specific lifecycle with grace periods
  - Enhanced deduplication mechanisms to prevent duplicate log collection while ensuring comprehensive coverage
  - Comprehensive test coverage including unit tests and deduplication validation
  This resolves the issue where job logs were being missed, particularly for fast-completing jobs or jobs that terminated before discovery. (@QuentinBisson)

- Fix `loki.source.journal` creation failing with an error when the journal file is not found. (@thampiotr)

- Fix graph UI so it generates correct URLs for components in `remotecfg` modules. (@patrickeasters)

- Fix panic in `loki.write` when component is shutting down and `external_labels` are configured. (@kalleep)

- Fix excessive debug logs always being emitted by `prometheus.exporter.mongodb`. (@kalleep)

v1.10.2
-----------------

### Bugfixes

- Fix issue in `prometheus.write.queue` causing inability to increase shard count if existing WAL data was present on start. (@kgeckhart)

- Fix issue with `loki.source.gcplog` when push messages sent by gcp pub/sub only includes `messageId`. (@kalleep)

v1.10.1
-----------------

### Bugfixes

- Fix issue with `faro.receiver` cors not allowing X-Scope-OrgID and traceparent headers. (@mar4uk)

- Fix issues with propagating cluster peers change notifications to components configured with remotecfg. (@dehaansa)

- Fix issues with statistics reporter not including components only configured with remotecfg. (@dehaansa)

- Fix issues with `prometheus.exporter.windows` not propagating `dns` collector config. (@dehaansa)

- Fixed a bug in `prometheus.write.queue` which caused retries even when `max_retry_attempts` was set to `0`. (@ptodev)

- Fixed a bug in `prometheus.write.queue` which caused labelling issues when providing more than one label in `external_labels`. (@dehaansa)

- Add `application_host` and `network_inter_zone` features to `beyla.ebpf` component. (@marctc)

- Fix issues in `loki.process` where `stage.multiline` did not pass through structured metadata. (@jan-mrm)

- Fix URLs in the Windows installer being wrapped in quotes. (@martincostello)

- Fixed an issue where certain `otelcol.*` components could prevent Alloy from shutting down when provided invalid configuration. (@thampiotr)

v1.10.0
-----------------

### Breaking changes

- Removing the `nanoserver-1809` container image for Windows 2019. (@ptodev)
  This is due to the deprecation of `windows-2019` GitHub Actions runners.
  The `windowsservercore-ltsc2022` Alloy image is still being published to DockerHub.

### Bugfixes

- Upgrade `otelcol` components from OpenTelemetry v0.126.0 to v0.128.0 (@korniltsev, @dehaansa)
  - [`otelcol.exporter.kafka`]: Allow kafka exporter to produce to topics based on metadata key values.
  - [`otelcol.receiver.kafka`]: Enforce a backoff mechanism on non-permanent errors, such as when the queue is full.
  - [`otelcol.receiver.kafka`]: Don't restart the Kafka consumer on failed errors when message marking is enabled for them.
  - [`otelcol.exporter.datadog`]: Fix automatic intial point dropping when converting cumulative monotonic sum metrics.
  - [`otelcol.exporter.datadog`]: config `tls::insecure_skip_verify` is now taken into account in metrics path.
  - [`otelcol.exporter.datadog`]: Correctly treat summary counts as cumulative monotonic sums instead of cumulative non-monotonic sums.
  - [`otelcol.connector.spanmetrics`]: Fix bug causing span metrics calls count to be always 0 when using delta temporality.
  - [`otelcol.exporter.splunkhec`]: Treat HTTP 403 Forbidden as a permanent error.

### Features

- (_Experimental_) Add an `array.group_by` stdlib function to group items in an array by a key. (@wildum)
- Add the `otelcol.exporter.faro` exporter to export traces and logs to Faro endpoint. (@mar4uk)
- Add the `otelcol.receiver.faro` receiver to receive traces and logs from the Grafana Faro Web SDK. (@mar4uk)

- Add entropy support for `loki.secretfilter` (@romain-gaillard)

### Enhancements

- Add `hash_string_id` argument to `foreach` block to hash the string representation of the pipeline id instead of using the string itself. (@wildum)

- Update `async-profiler` binaries for `pyroscope.java` to 4.0-87b7b42 (@github-hamza-bouqal)

- (_Experimental_) Additions to experimental `database_observability.mysql` component:
  - Add `explain_plan` collector to `database_observability.mysql` component. (@rgeyer)
  - `locks`: addition of data locks collector (@gaantunes @fridgepoet)
  - `query_sample` collector is now enabled by default (@matthewnolf)
  - `query_tables` collector now deals better with truncated statements (@cristiangreco)

- (_Experimental_) `prometheus.write.queue` add support for exemplars. (@dehaansa)

- (_Experimental_) `prometheus.write.queue` initialize queue metrics that are seconds values as time.Now, not 0. (@dehaansa)

- Update secret-filter gitleaks.toml from v8.19.0 to v8.26.0 (@andrejshapal)

- Wire in survey block for beyla.ebpf component. (@grcevski, @tpaschalis)

- Upgrade `otelcol` components from OpenTelemetry v0.126.0 to v0.128.0 (@korniltsev, @dehaansa)
  - [`otelcol.processor.resourcedetection`]: Add additional OS properties to resource detection: `os.build.id` and `os.name`.
  - [`otelcol.processor.resourcedetection`]: Add `host.interface` resource attribute to `system` detector.
  - [`otelcol.exporter.kafka`]: Fix Snappy compression codec support for the Kafka exporter.
  - [`otelcol.receiver.filelog`]: Introduce `utf8-raw` encoding to avoid replacing invalid bytes with \uFFFD when reading UTF-8 input.
  - [`otelcol.processor.k8sattributes`]: Support extracting labels and annotations from k8s Deployments.
  - [`otelcol.processor.k8sattributes`]: Add option to configure automatic service resource attributes.
  - [`otelcol.exporter.datadog`]: Adds `hostname_detection_timeout` configuration option for Datadog Exporter and sets default to 25 seconds.
  - [`otelcol.receiver.datadog`]: Address semantic conventions noncompliance and add support for http/db.
  - [`otelcol.exporter.awss3`]: Add the retry mode, max attempts and max backoff to the settings.

- Add `enable_tracing` attribute to `prometheus.exporter.snowflake` component to support debugging issues. (@dehaansa)

- Add support for `conditions` and statement-specific `error_mode` in `otelcol.processor.transform`. (@ptodev)

- Add `storage` and `start_from` args to cloudwatch logs receiver. (@boernd)

- Reduced allocation in Loki processing pipelines. (@thampiotr)

- Update the `prometheus.exporter.postgres` component with latest changes and bugfixes for Postgres17 (@cristiangreco)

- Add `tail_from_end` argument to `loki.source.podlogs` to optionally start reading from the end of a log stream for newly discovered pods. (@harshrai654)

- Remove limitation in `loki.source.file` when `legacy_position_file` is unset. Alloy can now recover legacy positions even if labels are added. (@kalleep)

### Bugfixes

- Fix path for correct injection of version into constants at build time. (@adlotsof)

- Propagate the `-feature.community-components.enabled` flag for remote
  configuration components. (@tpaschalis)

- Fix extension registration for `otelcol.receiver.splunkhec` auth extensions. (@dehaansa)

### Other changes

- Mark `pyroscope.receive_http` and `pyroscope.relabel` components as GA. (@marcsanmi)

- Upgrade `otelcol.exporter.windows` to v0.30.8 to get bugfixes and fix `update` collector support. (@dehaansa)

- Add `User-Agent` header to remotecfg requests. (@tpaschalis)

v1.9.2
-----------------

### Bugfixes

- Send profiles concurrently from `pyroscope.ebpf`. (@korniltsev)

- Fix the `validate` command not understanding the `livedebugging` block. (@dehaansa)

- Fix invalid class names in python profiles obtained with `pyroscope.ebpf`. (@korniltsev)

- Fixed a bug which prevented non-secret optional secrets to be passed in as `number` arguments. (@ptodev)

- For CRD-based components (`prometheus.operator.*`), retry initializing informers if the apiserver request fails. This rectifies issues where the apiserver is not reachable immediately after node restart. (@dehaansa)

### Other changes

-  Add no-op blocks and attributes to the `prometheus.exporter.windows` component (@ptodev).
   Version 1.9.0 of Alloy removed the `msmq` block, as well as the `enable_v2_collector`,
   `where_clause`, and `use_api` attributes in the `service` block.
   This made it difficult for users to upgrade, so those attributes have now been made a no-op instead of being removed.

v1.9.1
-----------------

### Features

- Update the `prometheus.exporter.windows` component to version v0.30.7. This adds new metrics to the `dns` collector. (@dehaansa)

### Bugfixes

- Update the `prometheus.exporter.windows` component to version v0.30.7. This fixes an error with the exchange collector and terminal_services collector (@dehaansa)

- Fix `loki.source.firehose` to propagate specific cloudwatch event timestamps when useIncomingTs is set to true. (@michaelPotter)

- Fix elevated CPU usage when using some `otelcol` components due to debug logging. (@thampiotr)

### Other changes

- Upgrade `otelcol` components from OpenTelemetry v0.125.0 to v0.126.0 (@dehaansa):
  - [`pkg/ottl`] Add support for `HasPrefix` and `HasSuffix` functions.
  - [`pkg/configtls`] Add trusted platform module (TPM) support to TLS authentication for all `otelcol` components supporting TLS.
  - [`otelcol.connector.spanmetrics`] Add `calls_dimension` and `histogram:dimension` blocks for configuring additional dimensions for `traces.span.metrics.calls` and `traces.span.metrics.duration` metrics.
  - [`otelcol.exporter.datadog`] Enable `instrumentation_scope_metadata_as_tags` by default.
  - [`otelcol.exporter.kafka`] support configuration of `compression` `level` in producer configuration.
  - [`otelcol.processor.tailsampling`] `invert sample` and `inverted not sample` decisions deprecated, use the `drop` policy instead to explicitly not sample traces.
  - [`otelcol.receiver.filelog`] support `compression` value of `auto` to automatically detect file compression type.

v1.9.0
-----------------

### Breaking changes

- The `prometheus.exporter.windows` component has been update to version v0.30.6. This update includes a significant rework of the exporter and includes some breaking changes. (@dehaansa)
  - The `msmq` and `service` collectors can no longer be configured with a WMI where clause. Any filtering previously done in a where clause will need to be done in a `prometheus.relabel` component.
  - The `service` collector no longer provides `enable_v2_collector` and `use_api` configuration options.
  - The `mscluster_*` and `netframework_*` collectors are now replaced with one `mscluster` and `netframework` collector that allows you to enable the separate metric groupings individually.
  - The `teradici_pcoip` and `vmware_blast` collectors have been removed from the exporter.

- The `prometheus.exporter.oracledb` component now embeds the [`oracledb_exporter from oracle`](https://github.com/oracle/oracle-db-appdev-monitoring) instead of the deprecated [`oracledb_exporter from iamseth`](https://github.com/iamseth/oracledb_exporter) for collecting metrics from an OracleDB server: (@wildum)
  - The arguments `username`, `password`, `default_metrics`, and `custom_metrics` are now supported.
  - The previously undocumented argument `custom_metrics` is now expecting a list of paths to custom metrics files.
  - The following metrics are no longer available by default: oracledb_sessions_activity, oracledb_tablespace_free_bytes

- (_Experimental_) The `enable_context_propagation` argument in `beyla.ebpf` has been replaced with the `context_propagation` argument.
  Set `enable_context_propagation` to `all` to get the same behaviour as `enable_context_propagation` being set to `true`.

### Features

- Bump snmp_exporter and embedded modules in `prometheus.exporter.snmp` to v0.29.0, add cisco_device module support (@v-zhuravlev)

- Add the `otelcol.storage.file` extension to support persistent sending queues and `otelcol.receiver.filelog` file state tracking between restarts. (@dehaansa)

- Add `otelcol.exporter.googlecloud` community component to export metrics, traces, and logs to Google Cloud. (@motoki317)

- Add support to configure basic authentication for alloy http server. (@kalleep)

- Add `validate` command to alloy that will perform limited validation of alloy configuration files. (@kalleep)

- Add support to validate foreach block when using `validate` command. (@kalleep)

- Add `otelcol.receiver.splunkhec` component to receive events in splunk hec format and forward them to other `otelcol.*` components. (@kalleep)

- Add support for Mimir federated rule groups in `mimir.rules.kubernetes` (@QuentinBisson)

### Enhancements

- `prometheus.exporter.windows` has been significantly refactored upstream and includes new collectors like `filetime`, `pagefile`, `performancecounter`, `udp`, and `update` as well as new configuration options for existing collectors. (@dehaansa)

- `prometheus.exporter.mongodb` now offers fine-grained control over collected metrics with new configuration options. (@TeTeHacko)

- Add binary version to constants exposed in configuration file syntatx. (@adlots)

- Update `loki.secretfilter` to include metrics about redactions (@kelnage)

- (_Experimental_) Various changes to the experimental component `database_observability.mysql`:
  - `schema_table`: add support for index expressions (@cristiangreco)
  - `query_sample`: enable opt-in support to extract unredacted sql query (sql_text) (@matthewnolf)
  - `query_tables`: improve queries parsing (@cristiangreco)
  - make tidbparser the default choice (@cristiangreco)
  - `query_sample`: better handling of timer overflows (@fridgepoet)
  - collect metrics on enabled `performance_schema.setup_consumers` (@fridgepoet)
  - `query_sample`: base log entries on calculated timestamp from rows, not now() (@fridgepoet)
  - `query_sample`: check digest is not null (@cristiangreco)
  - `query_sample`: add additional logs for wait events (@fridgepoet)
  - make tidb the default and only sql parser

- Mixin dashboards improvements: added minimum cluster size to Cluster Overview dashboard, fixed units in OpenTelemetry dashboard, fixed slow components evaluation time units in Controller dashboard and updated Prometheus dashboard to correctly aggregate across instances. (@thampiotr)

- Reduced the lag time during targets handover in a cluster in `prometheus.scrape` components by reducing thread contention. (@thampiotr)

- Pretty print diagnostic errors when using `alloy run` (@kalleep)

- Add `labels_from_groups` attribute to `stage.regex` in `loki.process` to automatically add named capture groups as labels. (@harshrai654)

- The `loki.rules.kubernetes` component now supports adding extra label matchers
  to all queries discovered via `PrometheusRule` CRDs. (@QuentinBisson)

-  Add optional `id` field to `foreach` block to generate more meaningful component paths in metrics by using a specific field from collection items. (@harshrai654)

- The `mimir.rules.kubernetes` component now supports adding extra label matchers
  to all queries discovered via `PrometheusRule` CRDs by extracting label values defined on the `PrometheusRule`. (@QuentinBisson)

- Fix validation logic in `beyla.ebpf` component to ensure that either metrics or traces are enabled. (@marctc)

- Improve `foreach` UI and add graph support for it. (@wildum)

- Update statsd_exporter to v0.28.0, most notable changes: (@kalleep)
  - [0.23.0] Support experimental native histograms.
  - [0.24.1] Support scaling parameter in mapping.
  - [0.26.0] Add option to honor original labels from event tags over labels specified in mapping configuration.
  - [0.27.1] Support dogstatsd extended aggregation
  - [0.27.2] Fix panic on certain invalid lines

- Upgrade `beyla.ebpf` to v2.2.4-alloy. The full list of changes can be found in the [Beyla release notes](https://github.com/grafana/beyla/releases/tag/v2.2.4-alloy). (@grcevski)

### Bugfixes

- Fix `otelcol.receiver.filelog` documentation's default value for `start_at`. (@petewall)

- Fix `pyroscope.scrape` scraping godeltaprof profiles. (@korniltsev)

- Fix [#3386](https://github.com/grafana/alloy/issues/3386) lower casing scheme in `prometheus.operator.scrapeconfigs`. (@alex-berger)

- Fix [#3437](https://github.com/grafana/alloy/issues/3437) Component Graph links now follow `--server.http.ui-path-prefix`. (@solidcellaMoon)

- Fix a bug in the `foreach` preventing the UI from showing the components in the template when the block was re-evaluated. (@wildum)

- Fix alloy health handler so header is written before response body. (@kalleep)

- Fix `prometheus.exporter.unix` to pass hwmon config correctly. (@kalleep)

- Fix [#3408](https://github.com/grafana/alloy/issues/3408) `loki.source.docker` can now collect logs from containers not in the running state. (@adamamsmith)

### Other changes

- Update the zap logging adapter used by `otelcol` components to log arrays and objects. (@dehaansa)

- Updated Windows install script to add DisplayVersion into registry on install (@enessene)

- Update Docker builds to install latest Linux security fixes on top of base image (@jharvey10)

- Reduce Docker image size slightly by consolidating some RUN layers (@AchimGrolimund)

- RPM artifacts in Alloy GitHub releases are no longer signed.
  The artifacts on the `https://rpm.grafana.com` repository used by the `yum` package manager will continue to be signed. (@ptodev)

- Upgrade `otelcol` components from OpenTelemetry v0.122.0 to v0.125.0 (@ptodev):
  - [`pkg/ottl`] Enhance the Decode OTTL function to support all flavors of Base64.
  - [`otelcol.processor.resourcedetection`] Adding the `os.version` resource attribute to system processor.
  - [`otelcol.auth.bearer`] Allow the header name to be customized.
  - [`otelcol.exporter.awss3`] Add a new `sending_queue` feature.
  - [`otelcol.exporter.awss3`] Add a new `timeout` argument.
  - [`otelcol.exporter.awss3`] Add a new `resource_attrs_to_s3` configuration block.
  - [`otelcol.exporter.awss3`] Fixes an issue where the AWS S3 Exporter was forcing an ACL to be set, leading to unexpected behavior in S3 bucket permissions.
  - [`otelcol.connector.spanmetrics`] A new `include_instrumentation_scope` configuration argument.
  - [`otelcol.connector.spanmetrics`] Initialise new `calls_total` metrics at 0.
  - [`otelcol.connector.spanmetrics`] A new `aggregation_cardinality_limit` configuration argument
    to limit the number of unique combinations of dimensions that will be tracked for metrics aggregation.
  - [`otelcol.connector.spanmetrics`] Deprecate the unused argument `dimensions_cache_size`.
  - [`otelcol.connector.spanmetrics`] Moving the start timestamp (and last seen timestamp) from the resourceMetrics level to the individual metrics level.
    This will ensure that each metric has its own accurate start and last seen timestamps, regardless of its relationship to other spans.
  - [`otelcol.processor.k8sattributes`] Add option to configure automatic resource attributes - with annotation prefix.
    Implements [Specify resource attributes using Kubernetes annotations](https://github.com/open-telemetry/semantic-conventions/blob/main/docs/non-normative/k8s-attributes.md#specify-resource-attributes-using-kubernetes-annotations).
  - [`otelcol.connector.servicegraph`] Change `database_name_attribute` to accept a list of values.
  - [`otelcol.exporter.kafka`, `otelcol.receiver.kafka`] Deprecating the `auth` > `plain_text` block. Use `auth` > `sasl` with `mechanism` set to `PLAIN` instead.
  - [`otelcol.exporter.kafka`, `otelcol.receiver.kafka`] Deprecating the `topic` argument. Use `logs` > `topic`, `metrics` > `topic`, or `traces` > `topic` instead.
  - [`otelcol.exporter.kafka`, `otelcol.receiver.kafka`] Deprecate the `auth` > `tls` block. Use the top-level `tls` block instead.
  - [`otelcol.receiver.kafka`] Add max_fetch_wait config setting.
    This setting allows you to specify the maximum time that the broker will wait for min_fetch_size bytes of data
    to be available before sending a response to the client.
  - [ `otelcol.receiver.kafka`] Add support for configuring Kafka consumer rebalance strategy and group instance ID.

v1.8.3
-----------------

### Bugfixes

- Fix `mimir.rules.kubernetes` panic on non-leader debug info retrieval (@TheoBrigitte)

- Fix detection of the "streams limit exceeded" error in the Loki client so that metrics are correctly labeled as `ReasonStreamLimited`. (@maratkhv)

- Fix `loki.source.file` race condition that often lead to panic when using `decompression`. (@kalleep)

- Fix deadlock in `loki.source.file` that can happen when targets are removed. (@kalleep)

- Fix `loki.process` to emit valid logfmt. (@kalleep)

v1.8.2
-----------------

### Bugfixes

- Fix `otelcol.exporter.prometheus` dropping valid exemplars. (@github-vincent-miszczak)

- Fix `loki.source.podlogs` not adding labels `__meta_kubernetes_namespace` and `__meta_kubernetes_pod_label_*`. (@kalleep)

v1.8.1
-----------------

### Bugfixes

- `rfc3164_default_to_current_year` argument was not fully added to `loki.source.syslog` (@dehaansa)

- Fix issue with `remoteCfg` service stopping immediately and logging noop error if not configured (@dehaansa)

- Fix potential race condition in `remoteCfg` service metrics registration (@kalleep)

- Fix panic in `prometheus.exporter.postgres` when using minimal url as data source name. (@kalleep)

v1.8.0
-----------------

### Breaking changes

- Removed `open_port` and `executable_name` from top level configuration of Beyla component. Removed `enabled` argument from `network` block. (@marctc)

- Breaking changes from the OpenTelemetry Collector v0.122 update: (@wildum)
  - `otelcol.exporter.splunkhec`: `min_size_items` and `max_size_items` were replaced by `min_size`, `max_size` and `sizer` in the `batcher` block to allow
  users to configure the size of the batch in a more flexible way.
  - The telemetry level of Otel components is no longer configurable. The `level` argument in the `debug_metrics` block is kept to avoid breaking changes but it is not used anymore.
  - `otelcol.processor.tailsampling` changed the unit of the decision timer metric from microseconds to milliseconds. (change unit of otelcol_processor_tail_sampling_sampling_decision_timer_latency)
  - `otelcol.processor.deltatocumulative`: rename `otelcol_deltatocumulative_datapoints_processed` to `otelcol_deltatocumulative_datapoints` and remove the metrics `otelcol_deltatocumulative_streams_evicted`, `otelcol_deltatocumulative_datapoints_dropped` and `otelcol_deltatocumulative_gaps_length`.
  - The `regex` attribute was removed from `otelcol.processor.k8sattributes`. The extract-patterns function from `otelcol.processor.transform` can be used instead.
  - The default value of `metrics_flush_interval` in `otelcol.connector.servicegraph` was changed from `0s` to `60s`.
  - `s3_partition` in `otelcol.exporter.awss3` was replaced by `s3_partition_format`.

- (_Experimental_) `prometheus.write.queue` metric names changed to align better with prometheus standards. (@mattdurham)

### Features

- Add `otelcol.receiver.awscloudwatch` component to receive logs from AWS CloudWatch and forward them to other `otelcol.*` components. (@wildum)
- Add `loki.enrich` component to enrich logs using labels from `discovery.*` components. (@v-zhuravlev)
- Add string concatenation for secrets type (@ravishankar15)
- Add support for environment variables to OpenTelemetry Collector config. (@jharvey10)
- Replace graph in Alloy UI with a new version that supports modules and data flow visualization. (@wildum)
- Added `--cluster.wait-for-size` and `--cluster.wait-timeout` flags which allow to specify the minimum cluster size
  required before components that use clustering begin processing traffic to ensure adequate cluster capacity is
  available. (@thampiotr)
- Add `trace_printer` to `beyla.ebpf` component to print trace information in a specific format. (@marctc)
- Add support for live debugging and graph in the UI for components imported via remotecfg. (@wildum)

### Enhancements

- Add the ability to set user for Windows Service with silent install (@dehaansa)

- Add livedebugging support for structured_metadata in `loki.process` (@dehaansa)

- (_Public Preview_) Add a `--windows.priority` flag to the run command, allowing users to set windows process priority for Alloy. (@dehaansa)

- (_Experimental_) Adding a new `prometheus.operator.scrapeconfigs` which discovers and scrapes [ScrapeConfig](https://prometheus-operator.dev/docs/developer/scrapeconfig/) Kubernetes resources. (@alex-berger)

- Add `rfc3164_default_to_current_year` argument to `loki.source.syslog` (@dehaansa)

- Add `connection_name` support for `prometheus.exporter.mssql` (@bck01215)

- Add livedebugging support for `prometheus.scrape` (@ravishankar15, @wildum)

- Have `loki.echo` log the `entry_timestamp` and `structured_metadata` for any loki entries received (@dehaansa)

- Bump snmp_exporter and embedded modules in `prometheus.exporter.snmp` to v0.28.0 (@v-zhuravlev)

- Update mysqld_exporter to v0.17.2, most notable changes: (@cristiangreco)
  - [0.17.1] Add perf_schema quantile columns to collector
  - [0.17.1] Fix database quoting problem in collector 'info_schema.tables'
  - [0.17.1] Use SUM_LOCK_TIME and SUM_CPU_TIME with mysql >= 8.0.28
  - [0.17.1] Fix query on perf_schema.events_statements_summary_by_digest
  - [0.17.2] Fix query on events_statements_summary_by_digest for mariadb

- Added additional backwards compatibility metrics to `prometheus.write.queue`. (@mattdurham)

- Add new stdlib functions encoding.to_json (@ravishankar15)

- Added OpenTelemetry logs and metrics support to Alloy mixin's dashboards and alerts. (@thampiotr)

- Add support for proxy and headers in `prometheus.write.queue`. (@mattdurham)

- Added support for switching namespace between authentication and kv retrieval to support Vault Enterprise (@notedop)

- (_Experimental_) Various changes to the experimental component `database_observability.mysql`:
  - `query_sample`: better handling of truncated queries (@cristiangreco)
  - `query_sample`: add option to use TiDB sql parser (@cristiangreco)
  - `query_tables`: rename collector from `query_sample` to better reflect responsibility (@matthewnolf)
  - `query_sample`: add new collector that replaces previous implementation to collect more detailed sample information (@matthewnolf)
  - `query_sample`: refactor parsing of truncated queries (@cristiangreco)

- Add labels validation in `pyroscope.write` to prevent duplicate labels and invalid label names/values. (@marcsanmi)

- Reduced lock contention in `prometheus.scrape` component (@thampiotr)

- Support converting otel config which uses a common receiver across pipelines with different names. (@wildum)

- Reduce CPU usage of the `loki.source.podlogs` component when pods logs target lots of pods (@QuentinBisson)

- Add error body propagation in `pyroscope.write`, for `/ingest` calls. (@simonswine)

- Add `tenant` label to remaining `loki_write_.+` metrics (@towolf)

- Removed syntax highlighting from the component details UI view to improve
  rendering performance. (@tpaschalis)

- A new `grafana/alloy:vX.Y.Z-windowsservercore-ltsc2022` Docker image is now published on DockerHub. (@ptodev)

### Bugfixes

- Fix deadlocks in `loki.source.file` when tailing fails (@mblaschke)
- Add missing RBAC permission for ScrapeConfig (@alex-berger)

- Fixed an issue in the `mimir.rules.kubernetes` component that would keep the component as unhealthy even when it managed to start after temporary errors (@nicolasvan)

- Allow kafka exporter to attempt to connect even if TLS enabled but cert & key are not specified (@dehaansa)

- Fixed bug where all resources were not being collected from `prometheus.exporter.azure` when using `regions` (@kgeckhart)

- Fix panic in `loki.source.file` when the tailer had no time to run before the runner was stopped (@wildum)

### Other changes

- Upgrading to Prometheus v2.55.1. (@ptodev)
  - Added a new `http_headers` argument to many `discovery` and `prometheus` components.
  - Added a new `scrape_failure_log_file` argument to `prometheus.scrape`.

- Non-breaking changes from the OpenTelemetry Collector v0.122 update: (@wildum)
  - `otelcol.processor.transform` has a new `statements` block for transformations which don't require a context to be specified explicitly.
  - `otelcol.receiver.syslog` has a new `on_error` argument to specify the action to take when an error occurs while receiving logs.
  - `otelcol.processor.resourcedetection` now supports `dynatrace` as a resource detector.
  - `otelcol.receiver.kafka` has a new `error_backoff` block to configure how failed requests are retried.
  - `otelcol.receiver.vcenter` has three new metrics `vcenter.vm.cpu.time`, `vcenter.vm.network.broadcast.packet.rate` and `vcenter.vm.network.multicast.packet.rate`.
  - `otelcol.exporter.awss3` has two new arguments `acl` and `storage_class`.
  - `otelcol.auth.headers` headers can now be populated using Authentication metadata using from_attribute

- Change the stability of the `beyla.ebpf` component from "public preview" to "generally available". (@marctc)

- The ingest API of `pyroscope.receive_http` no longer forwards all received headers, instead only passes through the `Content-Type` header. (@simonswine)

v1.7.5
-----------------

### Enhancements

- Set zstd as default compression for `prometheus.write.queue`. (@mattdurham)

v1.7.4
-----------------

### Bugfixes

- Revert the changes to `loki.source.file` from release v1.7.0. These changes introduced a potential deadlock. (@dehaansa)

v1.7.3
-----------------

### Breaking changes

- Fixed the parsing of selections, application and network filter blocks for Beyla. (@raffaelroquetto)

### Enhancements

- Add the `stat_checkpointer` collector in `prometheus.exporter.postgres` (@dehaansa)

### Bugfixes

- Update the `prometheus.exporter.postgres` component to correctly support Postgres17 when `stat_bgwriter` collector is enabled (@dehaansa)

- Fix `remoteCfg` logging and metrics reporting of `errNotModified` as a failure (@zackman0010)


v1.7.2
-----------------

### Bugfixes

- Fixed an issue where the `otelcol.exporter.awss3` could not be started with the `sumo_ic` marshaler. (@wildum)

- Update `jfr-parser` dependency to v0.9.3 to fix jfr parsing issues in `pyroscope.java`. (@korniltsev)

- Fixed an issue where passing targets from some standard library functions was failing with `target::ConvertFrom` error. (@thampiotr)

- Fixed an issue where indexing targets as maps (e.g. `target["foo"]`) or objects (e.g. `target.foo`) or using them with
  certain standard library functions was resulting in `expected object or array, got capsule` error under some
  circumstances. This could also lead to `foreach evaluation failed` errors when using the `foreach` configuration
  block. (@thampiotr)

- Update `prometheus.write.queue` to reduce memory fragmentation and increase sent throughput. (@mattdurham)

- Fixed an issue where the `otelcol.exporter.kafka` component would not start if the `encoding` was specific to a signal type. (@wildum)

v1.7.1
-----------------

### Bugfixes

- Fixed an issue where some exporters such as `prometheus.exporter.snmp` couldn't accept targets from other components
  with an error `conversion to '*map[string]string' is not supported"`. (@thampiotr)

- Enable batching of calls to the appender in `prometheus.write.queue` to reduce lock contention when scraping, which
  will lead to reduced scrape duration. (@mattdurham)

v1.7.0
-----------------

### Breaking changes

- (_Experimental_) In `prometheus.write.queue` changed `parallelism` from attribute to a block to allow for dynamic scaling. (@mattdurham)

- Remove `tls_basic_auth_config_path` attribute from `prometheus.exporter.mongodb` configuration as it does not configure TLS client
  behavior as previously documented.

- Remove `encoding` and `encoding_file_ext` from `otelcol.exporter.awss3` component as it was not wired in to the otel component and
  Alloy does not currently integrate the upstream encoding extensions that this would utilize.

### Features

- Add a `otelcol.receiver.tcplog` component to receive OpenTelemetry logs over a TCP connection. (@nosammai)

- (_Public preview_) Add `otelcol.receiver.filelog` component to read otel log entries from files (@dehaansa)

- (_Public preview_) Add a `otelcol.processor.cumulativetodelta` component to convert metrics from
  cumulative temporality to delta. (@madaraszg-tulip)

- (_Experimental_) Add a `stage.windowsevent` block in the `loki.process` component. This aims to replace the existing `stage.eventlogmessage`. (@wildum)

- Add `pyroscope.relabel` component to modify or filter profiles using Prometheus relabeling rules. (@marcsanmi)

- (_Experimental_) A new `foreach` block which starts an Alloy pipeline for each item inside a list. (@wildum, @thampiotr, @ptodev)

### Enhancements

- Upgrade to OpenTelemetry Collector v0.119.0 (@dehaansa):
  - `otelcol.processor.resourcedetection`: additional configuration for the `ec2` detector to configure retry behavior
  - `otelcol.processor.resourcedetection`: additional configuration for the `gcp` detector to collect Managed Instance Group attributes
  - `otelcol.processor.resourcedetection`: additional configuration for the `eks` detector to collect cloud account attributes
  - `otelcol.processor.resourcedetection`: add `kubeadm` detector to collect local cluster attributes
  - `otelcol.processor.cumulativetodelta`: add `metric_types` filtering options
  - `otelcol.exporter.awss3`: support configuring sending_queue behavior
  - `otelcol.exporter.otlphttp`: support configuring `compression_params`, which currently only includes `level`
  - `configtls`: opentelemetry components with tls config now support specifying TLS curve preferences
  - `sending_queue`: opentelemetry exporters with a `sending_queue` can now configure the queue to be `blocking`

- Add `go_table_fallback` arg to `pyroscope.ebpf` (@korniltsev)

- Memory optimizations in `pyroscope.scrape` (@korniltsev)

- Do not drop `__meta` labels in `pyroscope.scrape`. (@korniltsev)

- Add the possibility to export span events as logs in `otelcol.connector.spanlogs`. (@steve-hb)

- Add json format support for log export via faro receiver (@ravishankar15)

- (_Experimental_) Various changes to the experimental component `database_observability.mysql`:
  - `connection_info`: add namespace to the metric (@cristiangreco)
  - `query_sample`: better support for table name parsing (@cristiangreco)
  - `query_sample`: capture schema name for query samples (@cristiangreco)
  - `query_sample`: fix error handling during result set iteration (@cristiangreco)
  - `query_sample`: improve parsing of truncated queries (@cristiangreco)
  - `query_sample`: split out sql parsing logic to a separate file (@cristiangreco)
  - `schema_table`: add table columns parsing (@cristiagreco)
  - `schema_table`: correctly quote schema and table name in SHOW CREATE (@cristiangreco)
  - `schema_table`: fix handling of view table types when detecting schema (@matthewnolf)
  - `schema_table`: refactor cache config in schema_table collector (@cristiangreco)
  - Component: add enable/disable collector configurability to `database_observability.mysql`. This removes the `query_samples_enabled` argument, now configurable via enable/disable collector. (@fridgepoet)
  - Component: always log `instance` label key (@cristiangreco)
  - Component: better error handling for collectors (@cristiangreco)
  - Component: use labels for some indexed logs elements (@cristiangreco)

- Reduce CPU usage of `loki.source.windowsevent` by up to 85% by updating the bookmark file every 10 seconds instead of after every event and by
  optimizing the retrieval of the process name. (@wildum)

- Ensure consistent service_name label handling in `pyroscope.receive_http` to match Pyroscope's behavior. (@marcsanmi)

- Improved memory and CPU performance of Prometheus pipelines by changing the underlying implementation of targets (@thampiotr)

- Add `config_merge_strategy` in `prometheus.exporter.snmp` to optionally merge custom snmp config with embedded config instead of replacing. Useful for providing SNMP auths. (@v-zhuravlev)

- Upgrade `beyla.ebpf` to v2.0.4. The full list of changes can be found in the [Beyla release notes](https://github.com/grafana/beyla/releases/tag/v2.0.0). (@marctc)

### Bugfixes

- Fix log rotation for Windows in `loki.source.file` by refactoring the component to use the runner pkg. This should also reduce CPU consumption when tailing a lot of files in a dynamic environment. (@wildum)

- Add livedebugging support for `prometheus.remote_write` (@ravishankar15)

- Add livedebugging support for `otelcol.connector.*` components (@wildum)

- Bump snmp_exporter and embedded modules to 0.27.0. Add support for multi-module handling by comma separation and expose argument to increase SNMP polling concurrency for `prometheus.exporter.snmp`. (@v-zhuravlev)

- Add support for pushv1.PusherService Connect API in `pyroscope.receive_http`. (@simonswine)

- Fixed an issue where `loki.process` would sometimes output live debugging entries out-of-order (@thampiotr)

- Fixed a bug where components could be evaluated concurrently without the full context during a config reload (@wildum)

- Fixed locks that wouldn't be released in the remotecfg service if some errors occurred during the configuration reload (@spartan0x117)

- Fix issue with `prometheus.write.queue` that lead to excessive connections. (@mattdurham)

- Fixed a bug where `loki.source.awsfirehose` and `loki.source.gcplog` could
  not be used from within a module. (@tpaschalis)

- Fix an issue where Prometheus metric name validation scheme was set by default to UTF-8. It is now set back to the
  previous "legacy" scheme. An experimental flag `--feature.prometheus.metric-validation-scheme` can be used to switch
  it to `utf-8` to experiment with UTF-8 support. (@thampiotr)

### Other changes

- Upgrading to Prometheus v2.54.1. (@ptodev)
  - `discovery.docker` has a new `match_first_network` attribute for matching the first network
    if the container has multiple networks defined, thus avoiding collecting duplicate targets.
  - `discovery.ec2`, `discovery.kubernetes`, `discovery.openstack`, and `discovery.ovhcloud`
    add extra `__meta_` labels.
  - `prometheus.remote_write` supports Azure OAuth and Azure SDK authentication.
  - `discovery.linode` has a new `region` attribute, as well as extra `__meta_` labels.
  - A new `scrape_native_histograms` argument for `prometheus.scrape`.
    This is enabled by default and can be used to explicitly disable native histogram support.
    In previous versions of Alloy, native histogram support has also been enabled by default
    as long as `scrape_protocols` starts with `PrometheusProto`.

  - Change the stability of the `remotecfg` feature from "public preview" to "generally available". (@erikbaranowski)

v1.6.1
-----------------

## Bugs

- Resolve issue with Beyla starting. (@rafaelroquetto)

v1.6.0
-----------------

### Breaking changes

- Upgrade to OpenTelemetry Collector v0.116.0:
  - `otelcol.processor.tailsampling`: Change decision precedence when using `and_sub_policy` and `invert_match`.
    For more information, see the [release notes for Alloy 1.6][release-notes-alloy-1_6].

    [#33671]: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33671
    [release-notes-alloy-1_6]: https://grafana.com/docs/alloy/latest/release-notes/#v16

### Features

- Add support for TLS to `prometheus.write.queue`. (@mattdurham)

- Add `otelcol.receiver.syslog` component to receive otel logs in syslog format (@dehaansa)

- Add support for metrics in `otelcol.exporter.loadbalancing` (@madaraszg-tulip)

- Add `add_cloudwatch_timestamp` to `prometheus.exporter.cloudwatch` metrics. (@captncraig)

- Add support to `prometheus.operator.servicemonitors` to allow `endpointslice` role. (@yoyosir)

- Add `otelcol.exporter.splunkhec` allowing to export otel data to Splunk HEC (@adlotsof)

- Add `otelcol.receiver.solace` component to receive traces from a Solace broker. (@wildum)

- Add `otelcol.exporter.syslog` component to export logs in syslog format (@dehaansa)

- (_Experimental_) Add a `database_observability.mysql` component to collect mysql performance data. (@cristiangreco & @matthewnolf)

- Add `otelcol.receiver.influxdb` to convert influx metric into OTEL. (@EHSchmitt4395)

- Add a new `/-/healthy` endpoint which returns HTTP 500 if one or more components are unhealthy. (@ptodev)

### Enhancements

- Improved performance by reducing allocation in Prometheus write pipelines by ~30% (@thampiotr)

- Update `prometheus.write.queue` to support v2 for cpu performance. (@mattdurham)

- (_Experimental_) Add health reporting to `database_observability.mysql` component (@cristiangreco)

- Add second metrics sample to the support bundle to provide delta information (@dehaansa)

- Add all raw configuration files & a copy of the latest remote config to the support bundle (@dehaansa)

- Add relevant golang environment variables to the support bundle (@dehaansa)

- Add support for server authentication to otelcol components. (@aidaleuc)

- Update mysqld_exporter from v0.15.0 to v0.16.0 (including 2ef168bf6), most notable changes: (@cristiangreco)
  - Support MySQL 8.4 replicas syntax
  - Fetch lock time and cpu time from performance schema
  - Fix fetching tmpTables vs tmpDiskTables from performance_schema
  - Skip SPACE_TYPE column for MariaDB >=10.5
  - Fixed parsing of timestamps with non-zero padded days
  - Fix auto_increment metric collection errors caused by using collation in INFORMATION_SCHEMA searches
  - Change processlist query to support ONLY_FULL_GROUP_BY sql_mode
  - Add perf_schema quantile columns to collector

- Live Debugging button should appear in UI only for supported components (@ravishankar15)
- Add three new stdlib functions to_base64, from_URLbase64 and to_URLbase64 (@ravishankar15)
- Add `ignore_older_than` option for local.file_match (@ravishankar15)
- Add livedebugging support for discovery components (@ravishankar15)
- Add livedebugging support for `discover.relabel` (@ravishankar15)
- Performance optimization for live debugging feature (@ravishankar15)

- Upgrade `github.com/goccy/go-json` to v0.10.4, which reduces the memory consumption of an Alloy instance by 20MB.
  If Alloy is running certain otelcol components, this reduction will not apply. (@ptodev)
- improve performance in regexp component: call fmt only if debug is enabled (@r0ka)

- Update `prometheus.write.queue` library for performance increases in cpu. (@mattdurham)

- Update `loki.secretfilter` to be compatible with the new `[[rules.allowlists]]` gitleaks allowlist format (@romain-gaillard)

- Update `async-profiler` binaries for `pyroscope.java` to 3.0-fa937db (@aleks-p)

- Reduced memory allocation in discovery components by up to 30% (@thampiotr)

### Bugfixes

- Fix issue where `alloy_prometheus_relabel_metrics_processed` was not being incremented. (@mattdurham)

- Fixed issue with automemlimit logging bad messages and trying to access cgroup on non-linux builds (@dehaansa)

- Fixed issue with reloading configuration and prometheus metrics duplication in `prometheus.write.queue`. (@mattdurham)

- Updated `prometheus.write.queue` to fix issue with TTL comparing different scales of time. (@mattdurham)

- Fixed an issue in the `prometheus.operator.servicemonitors`, `prometheus.operator.podmonitors` and `prometheus.operator.probes` to support capitalized actions. (@QuentinBisson)

- Fixed an issue where the `otelcol.processor.interval` could not be used because the debug metrics were not set to default. (@wildum)

- Fixed an issue where `loki.secretfilter` would crash if the secret was shorter than the `partial_mask` value. (@romain-gaillard)

- Change the log level in the `eventlogmessage` stage of the `loki.process` component from `warn` to `debug`. (@wildum)

- Fix a bug in `loki.source.kafka` where the `topics` argument incorrectly used regex matching instead of exact matches. (@wildum)

### Other changes

- Change the stability of the `livedebugging` feature from "experimental" to "generally available". (@wildum)

- Use Go 1.23.3 for builds. (@mattdurham)

- Upgrade Beyla to v1.9.6. (@wildum)

- Upgrade to OpenTelemetry Collector v0.116.0:
  - `otelcol.receiver.datadog`: Return a json reponse instead of "OK" when a trace is received with a newer protocol version.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/35705
  - `otelcol.receiver.datadog`: Changes response message for `/api/v1/check_run` 202 response to be JSON and on par with Datadog API spec
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36029
  - `otelcol.receiver.solace`: The Solace receiver may unexpectedly terminate on reporting traces when used with a memory limiter processor and under high load.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/35958
  - `otelcol.receiver.solace`: Support converting the new `Move to Dead Message Queue` and new `Delete` spans generated by Solace Event Broker to OTLP.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36071
  - `otelcol.exporter.datadog`: Stop prefixing `http_server_duration`, `http_server_request_size` and `http_server_response_size` with `otelcol`.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36265
    These metrics can be from SDKs rather than collector. Stop prefixing them to be consistent with
    https://opentelemetry.io/docs/collector/internal-telemetry/#lists-of-internal-metrics
  - `otelcol.receiver.datadog`: Add json handling for the `api/v2/series` endpoint in the datadogreceiver.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36218
  - `otelcol.processor.span`: Add a new `keep_original_name` configuration argument
    to keep the original span name when extracting attributes from the span name.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36397
  - `pkg/ottl`: Respect the `depth` option when flattening slices using `flatten`.
    The `depth` option is also now required to be at least `1`.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36198
  - `otelcol.exporter.loadbalancing`: Shutdown exporters during collector shutdown. This fixes a memory leak.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36024
  - `otelcol.processor.k8sattributes`: New `wait_for_metadata` and `wait_for_metadata_timeout` configuration arguments,
    which block the processor startup until metadata is received from Kubernetes.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32556
  - `otelcol.processor.k8sattributes`: Enable the `k8sattr.fieldExtractConfigRegex.disallow` for all Alloy instances,
    to retain the behavior of `regex` argument in the `annotation` and `label` blocks.
    When the feature gate is "deprecated" in the upstream Collector, Alloy users will need to use the transform processor instead.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/25128
  - `otelcol.receiver.vcenter`: The existing code did not honor TLS settings beyond 'insecure'.
    All TLS client config should now be honored.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36482
  - `otelcol.receiver.opencensus`: Do not report error message when OpenCensus receiver is shutdown cleanly.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36622
  - `otelcol.processor.k8sattributes`: Fixed parsing of k8s image names to support images with tags and digests.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36145
  - `otelcol.exporter.loadbalancing`: Adding sending_queue, retry_on_failure and timeout settings to loadbalancing exporter configuration.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35378
  - `otelcol.exporter.loadbalancing`: The k8sresolver was triggering exporter churn in the way the change event was handled.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35658
  - `otelcol.processor.k8sattributes`: Override extracted k8s attributes if original value has been empty.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36466
  - `otelcol.exporter.awss3`: Upgrading to adopt aws sdk v2.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/36698
  - `pkg/ottl`: GetXML Converter now supports selecting text, CDATA, and attribute (value) content.
  - `otelcol.exporter.loadbalancing`: Adds a an optional `return_hostnames` configuration argument to the k8s resolver.
     https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/35411
  - `otelcol.exporter.kafka`, `otelcol.receiver.kafka`: Add a new `AWS_MSK_IAM_OAUTHBEARER` mechanism.
    This mechanism use the AWS MSK IAM SASL Signer for Go https://github.com/aws/aws-msk-iam-sasl-signer-go.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/32500

  - Use Go 1.23.5 for builds. (@wildum)

v1.5.1
-----------------

### Enhancements

- Logs from underlying clustering library `memberlist` are now surfaced with correct level (@thampiotr)

- Allow setting `informer_sync_timeout` in prometheus.operator.* components. (@captncraig)

- For sharding targets during clustering, `loki.source.podlogs` now only takes into account some labels. (@ptodev)

- Improve instrumentation of `pyroscope.relabel` component. (@marcsanmi)

### Bugfixes

- Fixed an issue in the `pyroscope.write` component to prevent TLS connection churn to Pyroscope when the `pyroscope.receive_http` clients don't request keepalive (@madaraszg-tulip)

- Fixed an issue in the `pyroscope.write` component with multiple endpoints not working correctly for forwarding profiles from `pyroscope.receive_http` (@madaraszg-tulip)

- Fixed a few race conditions that could lead to a deadlock when using `import` statements, which could lead to a memory leak on `/metrics` endpoint of an Alloy instance. (@thampiotr)

- Fix a race condition where the ui service was dependent on starting after the remotecfg service, which is not guaranteed. (@dehaansa & @erikbaranowski)

- Fixed an issue in the `otelcol.exporter.prometheus` component that would set series value incorrectly for stale metrics (@YusifAghalar)

- `loki.source.podlogs`: Fixed a bug which prevented clustering from working and caused duplicate logs to be sent.
  The bug only happened when no `selector` or `namespace_selector` blocks were specified in the Alloy configuration. (@ptodev)

- Fixed an issue in the `pyroscope.write` component to allow slashes in application names in the same way it is done in the Pyroscope push API (@marcsanmi)

- Fixed a crash when updating the configuration of `remote.http`. (@kinolaev)

- Fixed an issue in the `otelcol.processor.attribute` component where the actions `delete` and `hash` could not be used with the `pattern` argument. (@wildum)

- Fixed an issue in the `prometheus.exporter.postgres` component that would leak goroutines when the target was not reachable (@dehaansa)

v1.5.0
-----------------

### Breaking changes

- `import.git`: The default value for `revision` has changed from `HEAD` to `main`. (@ptodev)
  It is no longer allowed to set `revision` to `"HEAD"`, `"FETCH_HEAD"`, `"ORIG_HEAD"`, `"MERGE_HEAD"`, or `"CHERRY_PICK_HEAD"`.

- The Otel update to v0.112.0 has a few breaking changes:
  - [`otelcol.processor.deltatocumulative`] Change `max_streams` default value to `9223372036854775807` (max int).
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/35048
  - [`otelcol.connector.spanmetrics`] Change `namespace` default value to `traces.span.metrics`.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/34485
  - [`otelcol.exporter.logging`] Removed in favor of the `otelcol.exporter.debug`.
    https://github.com/open-telemetry/opentelemetry-collector/issues/11337

### Features

- Add support bundle generation via the API endpoint /-/support (@dehaansa)

- Add the function `path_join` to the stdlib. (@wildum)

- Add `pyroscope.receive_http` component to receive and forward Pyroscope profiles (@marcsanmi)

- Add support to `loki.source.syslog` for the RFC3164 format ("BSD syslog"). (@sushain97)

- Add support to `loki.source.api` to be able to extract the tenant from the HTTP `X-Scope-OrgID` header (@QuentinBisson)

- (_Experimental_) Add a `loki.secretfilter` component to redact secrets from collected logs.

- (_Experimental_) Add a `prometheus.write.queue` component to add an alternative to `prometheus.remote_write`
  which allowing the writing of metrics  to a prometheus endpoint. (@mattdurham)

- (_Experimental_) Add the `array.combine_maps` function to the stdlib. (@ptodev, @wildum)

### Enhancements

- The `mimir.rules.kubernetes` component now supports adding extra label matchers
  to all queries discovered via `PrometheusRule` CRDs. (@thampiotr)

- The `cluster.use-discovery-v1` flag is now deprecated since there were no issues found with the v2 cluster discovery mechanism. (@thampiotr)

- SNMP exporter now supports labels in both `target` and `targets` parameters. (@mattdurham)

- Add support for relative paths to `import.file`. This new functionality allows users to use `import.file` blocks in modules
  imported via `import.git` and other `import.file`. (@wildum)

- `prometheus.exporter.cloudwatch`: The `discovery` block now has a `recently_active_only` configuration attribute
  to return only metrics which have been active in the last 3 hours.

- Add Prometheus bearer authentication to a `prometheus.write.queue` component (@freak12techno)

- Support logs that have a `timestamp` field instead of a `time` field for the `loki.source.azure_event_hubs` component. (@andriikushch)

- Add `proxy_url` to `otelcol.exporter.otlphttp`. (@wildum)

- Allow setting `informer_sync_timeout` in prometheus.operator.* components. (@captncraig)

### Bugfixes

- Fixed a bug in `import.git` which caused a `"non-fast-forward update"` error message. (@ptodev)

- Do not log error on clean shutdown of `loki.source.journal`. (@thampiotr)

- `prometheus.operator.*` components: Fixed a bug which would sometimes cause a
  "failed to create service discovery refresh metrics" error after a config reload. (@ptodev)

### Other changes

- Small fix in UI stylesheet to fit more content into visible table area. (@defanator)

- Changed OTEL alerts in Alloy mixin to use success rate for tracing. (@thampiotr)

- Support TLS client settings for clustering (@tiagorossig)

- Add support for `not_modified` response in `remotecfg`. (@spartan0x117)

- Fix dead link for RelabelConfig in the PodLog documentation page (@TheoBrigitte)

- Most notable changes coming with the OTel update from v0.108.0 vo v0.112.0 besides the breaking changes: (@wildum)
  - [`http config`] Add support for lz4 compression.
    https://github.com/open-telemetry/opentelemetry-collector/issues/9128
  - [`otelcol.processor.interval`] Add support for gauges and summaries.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34803
  - [`otelcol.receiver.kafka`] Add possibility to tune the fetch sizes.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/34431
  - [`otelcol.processor.tailsampling`] Add `invert_match` to boolean attribute.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/34730
  - [`otelcol.receiver.kafka`] Add support to decode to `otlp_json`.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33627
  - [`otelcol.processor.transform`] Add functions `convert_exponential_histogram_to_histogram` and `aggregate_on_attribute_value`.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33824
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33423

v1.4.3
-----------------

### Bugfixes

- Fix an issue where some `faro.receiver` would drop multiple fields defined in `payload.meta.browser`, as fields were defined in the struct.

- `pyroscope.scrape` no longer tries to scrape endpoints which are not active targets anymore. (@wildum @mattdurham @dehaansa @ptodev)

- Fixed a bug with `loki.source.podlogs` not starting in large clusters due to short informer sync timeout. (@elburnetto-intapp)

- `prometheus.exporter.windows`: Fixed bug with `exclude` regular expression config arguments which caused missing metrics. (@ptodev)

v1.4.2
-----------------

### Bugfixes

- Update windows_exporter from v0.27.2 vo v0.27.3: (@jkroepke)
  - Fixes a bug where scraping Windows service crashes alloy

- Update yet-another-cloudwatch-exporter from v0.60.0 vo v0.61.0: (@morremeyer)
  - Fixes a bug where cloudwatch S3 metrics are reported as `0`

- Issue 1687 - otelcol.exporter.awss3 fails to configure (@cydergoth)
  - Fix parsing of the Level configuration attribute in debug_metrics config block
  - Ensure "optional" debug_metrics config block really is optional

- Fixed an issue with `loki.process` where `stage.luhn` and `stage.timestamp` would not apply
  default configuration settings correctly (@thampiotr)

- Fixed an issue with `loki.process` where configuration could be reloaded even if there
  were no changes. (@ptodev, @thampiotr)

- Fix issue where `loki.source.kubernetes` took into account all labels, instead of specific logs labels. Resulting in duplication. (@mattdurham)

v1.4.1
-----------------

### Bugfixes

- Windows installer: Don't quote Alloy's binary path in the Windows Registry. (@jkroepke)

v1.4.0
-----------------

### Security fixes

- Add quotes to windows service path to prevent path interception attack. [CVE-2024-8975](https://grafana.com/security/security-advisories/cve-2024-8975/) (@mattdurham)

### Breaking changes

- Some debug metrics for `otelcol` components have changed. (@thampiotr)
  For example, `otelcol.exporter.otlp`'s `exporter_sent_spans_ratio_total` metric is now `otelcol_exporter_sent_spans_total`.

- [otelcol.processor.transform] The functions `convert_sum_to_gauge` and `convert_gauge_to_sum` must now be used in the `metric` `context` rather than in the `datapoint` context.
  https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34567 (@wildum)

- Upgrade Beyla from 1.7.0 to 1.8.2. A complete list of changes can be found on the Beyla releases page: https://github.com/grafana/beyla/releases. (@wildum)
  It contains a few breaking changes for the component `beyla.ebpf`:
  - renamed metric `process.cpu.state` to `cpu.mode`
  - renamed metric `beyla_build_info` to `beyla_internal_build_info`

### Features

- Added Datadog Exporter community component, enabling exporting of otel-formatted Metrics and traces to Datadog. (@polyrain)
- (_Experimental_) Add an `otelcol.processor.interval` component to aggregate metrics and periodically
  forward the latest values to the next component in the pipeline.


### Enhancements

- Clustering peer resolution through `--cluster.join-addresses` flag has been
  improved with more consistent behaviour, better error handling and added
  support for A/AAAA DNS records. If necessary, users can temporarily opt out of
  this new behaviour with the `--cluster.use-discovery-v1`, but this can only be
  used as a temporary measure, since this flag will be disabled in future
  releases. (@thampiotr)

- Added a new panel to Cluster Overview dashboard to show the number of peers
  seen by each instance in the cluster. This can help diagnose cluster split
  brain issues. (@thampiotr)

- Updated Snowflake exporter with performance improvements for larger environments.
  Also added a new panel to track deleted tables to the Snowflake mixin. (@Caleb-Hurshman)
- Add a `otelcol.processor.groupbyattrs` component to reassociate collected metrics that match specified attributes
    from opentelemetry. (@kehindesalaam)

- Update windows_exporter to v0.27.2. (@jkroepke)
  The `smb.enabled_list` and `smb_client.enabled_list` doesn't have any effect anymore. All sub-collectors are enabled by default.

- Live debugging of `loki.process` will now also print the timestamp of incoming and outgoing log lines.
  This is helpful for debugging `stage.timestamp`. (@ptodev)

- Add extra validation in `beyla.ebpf` to avoid panics when network feature is enabled. (@marctc)

- A new parameter `aws_sdk_version_v2` is added for the cloudwatch exporters configuration. It enables the use of aws sdk v2 which has shown to have significant performance benefits. (@kgeckhart, @andriikushch)

- `prometheus.exporter.cloudwatch` can now collect metrics from custom namespaces via the `custom_namespace` block. (@ptodev)

- Add the label `alloy_cluster` in the metric `alloy_config_hash` when the flag `cluster.name` is set to help differentiate between
  configs from the same alloy cluster or different alloy clusters. (@wildum)

- Add support for discovering the cgroup path(s) of a process in `process.discovery`. (@mahendrapaipuri)

### Bugfixes

- Fix a bug where the scrape timeout for a Probe resource was not applied, overwriting the scrape interval instead. (@morremeyer, @stefanandres)

- Fix a bug where custom components don't always get updated when the config is modified in an imported directory. (@ante012)

- Fixed an issue which caused loss of context data in Faro exception. (@codecapitano)

- Fixed an issue where providing multiple hostnames or IP addresses
  via `--cluster.join-addresses` would only use the first provided value.
  (@thampiotr)

- Fixed an issue where providing `<hostname>:<port>`
  in `--cluster.join-addresses` would only resolve with DNS to a single address,
  instead of using all the available records. (@thampiotr)

- Fixed an issue where clustering peers resolution via hostname in `--cluster.join-addresses`
  resolves to duplicated IP addresses when using SRV records. (@thampiotr)

- Fixed an issue where the `connection_string` for the `loki.source.azure_event_hubs` component
  was displayed in the UI in plaintext. (@MorrisWitthein)

- Fix a bug in `discovery.*` components where old `targets` would continue to be
  exported to downstream components. This would only happen if the config
  for `discovery.*`  is reloaded in such a way that no new targets were
  discovered. (@ptodev, @thampiotr)

- Fixed bug in `loki.process` with `sampling` stage where all components use same `drop_counter_reason`. (@captncraig)

- Fixed an issue (see https://github.com/grafana/alloy/issues/1599) where specifying both path and key in the remote.vault `path`
  configuration could result in incorrect URLs. The `path` and `key` arguments have been separated to allow for clear and accurate
  specification of Vault secrets. (@PatMis16)

### Other

- Renamed standard library functions. Old names are still valid but are marked deprecated. (@wildum)

- Aliases for the namespaces are deprecated in the Cloudwatch exporter. For example: "s3" is not allowed, "AWS/S3" should be used. Usage of the aliases will generate warnings in the logs. Support for the aliases will be dropped in the upcoming releases. (@kgeckhart, @andriikushch)

- Update OTel from v0.105.0 vo v0.108.0: (@wildum)
  - [`otelcol.receiver.vcenter`] New VSAN metrics.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33556
  - [`otelcol.receiver.kafka`] Add `session_timeout` and `heartbeat_interval` attributes.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33082
  - [`otelcol.processor.transform`] Add `aggregate_on_attributes` function for metrics.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33334
  - [`otelcol.receiver.vcenter`] Enable metrics by default
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33607

- Updated the docker base image to Ubuntu 24.04 (Noble Numbat). (@mattiasa )

v1.3.4
-----------------

### Bugfixes

- Windows installer: Don't quote Alloy's binary path in the Windows Registry. (@jkroepke)

v1.3.2
-----------------

### Security fixes

- Add quotes to windows service path to prevent path interception attack. [CVE-2024-8975](https://grafana.com/security/security-advisories/cve-2024-8975/) (@mattdurham)

v1.3.1
-----------------

### Bugfixes

- Changed the cluster startup behaviour, reverting to the previous logic where
  a failure to resolve cluster join peers results in the node creating its own cluster. This is
  to facilitate the process of bootstrapping a new cluster following user feedback (@thampiotr)

- Fix a memory leak which would occur any time `loki.process` had its configuration reloaded. (@ptodev)

v1.3.0
-----------------

### Breaking changes

- [`otelcol.exporter.otlp`,`otelcol.exporter.loadbalancing`]: Change the default gRPC load balancing strategy.
  The default value for the `balancer_name` attribute has changed to `round_robin`
  https://github.com/open-telemetry/opentelemetry-collector/pull/10319

### Breaking changes to non-GA functionality

- Update Public preview `remotecfg` argument from `metadata` to `attributes`. (@erikbaranowski)

- The default value of the argument `unmatched` in the block `routes` of the component `beyla.ebpf` was changed from `unset` to `heuristic` (@marctc)

### Features

- Added community components support, enabling community members to implement and maintain components. (@wildum)

- A new `otelcol.exporter.debug` component for printing OTel telemetry from
  other `otelcol` components to the console. (@BarunKGP)

### Enhancements
- Added custom metrics capability to oracle exporter. (@EHSchmitt4395)

- Added a success rate panel on the Prometheus Components dashboard. (@thampiotr)

- Add namespace field to Faro payload (@cedricziel)

- Add the `targets` argument to the `prometheus.exporter.blackbox` component to support passing blackbox targets at runtime. (@wildum)

- Add concurrent metric collection to `prometheus.exporter.snowflake` to speed up collection times (@Caleb-Hurshman)

- Added live debugging support to `otelcol.processor.*` components. (@wildum)

- Add automatic system attributes for `version` and `os` to `remotecfg`. (@erikbaranowski)

- Added live debugging support to `otelcol.receiver.*` components. (@wildum)

- Added live debugging support to `loki.process`. (@wildum)

- Added live debugging support to `loki.relabel`. (@wildum)

- Added a `namespace` label to probes scraped by the `prometheus.operator.probes` component to align with the upstream Prometheus Operator setup. (@toontijtgat2)

- (_Public preview_) Added rate limiting of cluster state changes to reduce the
  number of unnecessary, intermediate state updates. (@thampiotr)

- Allow setting the CPU profiling event for Java Async Profiler in `pyroscope.java` component (@slbucur)

- Update windows_exporter to v0.26.2. (@jkroepke)

- `mimir.rules.kubernetes` is now able to add extra labels to the Prometheus rules. (@psychomantys)

- `prometheus.exporter.unix` component now exposes hwmon collector config. (@dtrejod)

- Upgrade from OpenTelemetry v0.102.1 to v0.105.0.
  - [`otelcol.receiver.*`] A new `compression_algorithms` attribute to configure which
    compression algorithms are allowed by the HTTP server.
    https://github.com/open-telemetry/opentelemetry-collector/pull/10295
  - [`otelcol.exporter.*`] Fix potential deadlock in the batch sender.
    https://github.com/open-telemetry/opentelemetry-collector/pull/10315
  - [`otelcol.exporter.*`] Fix a bug when the retry and timeout logic was not applied with enabled batching.
    https://github.com/open-telemetry/opentelemetry-collector/issues/10166
  - [`otelcol.exporter.*`] Fix a bug where an unstarted batch_sender exporter hangs on shutdown.
    https://github.com/open-telemetry/opentelemetry-collector/issues/10306
  - [`otelcol.exporter.*`] Fix small batch due to unfavorable goroutine scheduling in batch sender.
    https://github.com/open-telemetry/opentelemetry-collector/issues/9952
  - [`otelcol.exporter.otlphttp`] A new `cookies` block to store cookies from server responses and reuse them in subsequent requests.
    https://github.com/open-telemetry/opentelemetry-collector/issues/10175
  - [`otelcol.exporter.otlp`] Fixed a bug where the receiver's http response was not properly translating grpc error codes to http status codes.
    https://github.com/open-telemetry/opentelemetry-collector/pull/10574
  - [`otelcol.processor.tail_sampling`] Simple LRU Decision Cache for "keep" decisions.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33533
  - [`otelcol.processor.tail_sampling`] Fix precedence of inverted match in and policy.
    Previously if the decision from a policy evaluation was `NotSampled` or `InvertNotSampled`
    it would return a `NotSampled` decision regardless, effectively downgrading the result.
    This was breaking the documented behaviour that inverted decisions should take precedence over all others.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33671
  - [`otelcol.exporter.kafka`,`otelcol.receiver.kafka`] Add config attribute to disable Kerberos PA-FX-FAST negotiation.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/26345
  - [`OTTL`]: Added `keep_matching_keys` function to allow dropping all keys from a map that don't match the pattern.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32989
  - [`OTTL`]: Add debug logs to help troubleshoot OTTL statements/conditions
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33274
  - [`OTTL`]: Introducing `append` function for appending items into an existing array.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32141
  - [`OTTL`]: Introducing `Uri` converter parsing URI string into SemConv
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32433
  - [`OTTL`]: Added a Hex() converter function
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33450
  - [`OTTL`]: Added a IsRootSpan() converter function.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33729
  - [`otelcol.processor.probabilistic_sampler`]: Add Proportional and Equalizing sampling modes.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31918
  - [`otelcol.processor.deltatocumulative`]: Bugfix to properly drop samples when at limit.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33285
  - [`otelcol.receiver.vcenter`] Fixes errors in some of the client calls for environments containing multiple datacenters.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33735
  - [`otelcol.processor.resourcedetection`] Fetch CPU info only if related attributes are enabled.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/33774
  - [`otelcol.receiver.vcenter`] Adding metrics for CPU readiness, CPU capacity, and network drop rate.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33607
  - [`otelcol.receiver.vcenter`] Drop support for vCenter 6.7.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33607
  - [`otelcol.processor.attributes`] Add an option to extract value from a client address
    by specifying `client.address` value in the `from_context` field.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/34048
  - `otelcol.connector.spanmetrics`: Produce delta temporality span metrics with StartTimeUnixNano and TimeUnixNano values representing an uninterrupted series.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/31780

- Upgrade Beyla component v1.6.3 to v1.7.0
  - Reporting application process metrics
  - New supported protocols: SQL, Redis, Kafka
  - Several bugfixes
  - Full list of changes: https://github.com/grafana/beyla/releases/tag/v1.7.0

- Enable instances connected to remotecfg-compatible servers to Register
  themselves to the remote service. (@tpaschalis)

- Allow in-memory listener to work for remotecfg-supplied components. (@tpaschalis)

### Bugfixes

- Fixed a clustering mode issue where a fatal startup failure of the clustering service
  would exit the service silently, without also exiting the Alloy process. (@thampiotr)

- Fix a bug which prevented config reloads to work if a Loki `metrics` stage is in the pipeline.
  Previously, the reload would fail for `loki.process` without an error in the logs and the metrics
  from the `metrics` stage would get stuck at the same values. (@ptodev)


v1.2.1
-----------------

### Bugfixes

- Fixed an issue with `loki.source.kubernetes_events` not starting in large clusters due to short informer sync timeout. (@nrwiersma)

- Updated [ckit](https://github.com/grafana/ckit) to fix an issue with armv7 panic on startup when forming a cluster. (@imavroukakis)

- Fixed a clustering mode issue where a failure to perform static peers
  discovery did not result in a fatal failure at startup and could lead to
  potential split-brain issues. (@thampiotr)

### Other

- Use Go 1.22.5 for builds. (@mattdurham)

v1.2.0
-----------------

### Security fixes
- Fixes the following vulnerabilities (@ptodev):
  - [CVE-2024-35255](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2024-35255)
  - [CVE-2024-36129](https://avd.aquasec.com/nvd/2024/cve-2024-36129/)

### Breaking changes

- Updated OpenTelemetry to v0.102.1. (@mattdurham)
  - Components `otelcol.receiver.otlp`,`otelcol.receiver.zipkin`,`otelcol.extension.jaeger_remote_sampling`, and `otelcol.receiver.jaeger` setting `max_request_body_size`
    default changed from unlimited size to `20MiB`. This is due to [CVE-2024-36129](https://github.com/open-telemetry/opentelemetry-collector/security/advisories/GHSA-c74f-6mfw-mm4v).

### Breaking changes to non-GA functionality

- Update Public preview `remotecfg` to use `alloy-remote-config` instead of `agent-remote-config`. The
  API has been updated to use the term `collector` over `agent`. (@erikbaranowski)

- Component `otelcol.receiver.vcenter` removed `vcenter.host.network.packet.errors`, `vcenter.host.network.packet.count`, and
  `vcenter.vm.network.packet.count`.
  - `vcenter.host.network.packet.errors` replaced by `vcenter.host.network.packet.error.rate`.
  - `vcenter.host.network.packet.count` replaced by `vcenter.host.network.packet.rate`.
  - `vcenter.vm.network.packet.count` replaced by `vcenter.vm.network.packet.rate`.

### Features

- Add an `otelcol.exporter.kafka` component to send OTLP metrics, logs, and traces to Kafka.

- Added `live debugging` to the UI. Live debugging streams data as they flow through components for debugging telemetry data.
  Individual components must be updated to support live debugging. (@wildum)

- Added live debugging support for `prometheus.relabel`. (@wildum)

- (_Experimental_) Add a `otelcol.processor.deltatocumulative` component to convert metrics from
  delta temporality to cumulative by accumulating samples in memory. (@rfratto)

- (_Experimental_) Add an `otelcol.receiver.datadog` component to receive
  metrics and traces from Datadog. (@carrieedwards, @jesusvazquez, @alexgreenbank, @fedetorres93)

- Add a `prometheus.exporter.catchpoint` component to collect metrics from Catchpoint. (@bominrahmani)

- Add the `-t/--test` flag to `alloy fmt` to check if a alloy config file is formatted correctly. (@kavfixnel)

### Enhancements

- (_Public preview_) Add native histogram support to `otelcol.receiver.prometheus`. (@wildum)
- (_Public preview_) Add metrics to report status of `remotecfg` service. (@captncraig)

- Added `scrape_protocols` option to `prometheus.scrape`, which allows to
  control the preferred order of scrape protocols. (@thampiotr)

- Add support for configuring CPU profile's duration scraped by `pyroscope.scrape`. (@hainenber)

- `prometheus.exporter.snowflake`: Add support for RSA key-pair authentication. (@Caleb-Hurshman)

- Improved filesystem error handling when working with `loki.source.file` and `local.file_match`,
  which removes some false-positive error log messages on Windows (@thampiotr)

- Updates `processor/probabilistic_sampler` to use new `FailedClosed` field from OTEL release v0.101.0. (@StefanKurek)

- Updates `receiver/vcenter` to use new features and bugfixes introduced in OTEL releases v0.100.0 and v0.101.0.
  Refer to the [v0.100.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.100.0)
  and [v0.101.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.101.0) release
  notes for more detailed information.
  Changes that directly affected the configuration are as follows: (@StefanKurek)
  - The resource attribute `vcenter.datacenter.name` has been added and enabled by default for all resource types.
  - The resource attribute `vcenter.virtual_app.inventory_path` has been added and enabled by default to
    differentiate between resource pools and virtual apps.
  - The resource attribute `vcenter.virtual_app.name` has been added and enabled by default to differentiate
    between resource pools and virtual apps.
  - The resource attribute `vcenter.vm_template.id` has been added and enabled by default to differentiate between
    virtual machines and virtual machine templates.
  - The resource attribute `vcenter.vm_template.name` has been added and enabled by default to differentiate between
    virtual machines and virtual machine templates.
  - The metric `vcenter.cluster.memory.used` has been removed.
  - The metric `vcenter.vm.network.packet.drop.rate` has been added and enabled by default.
  - The metric `vcenter.cluster.vm_template.count` has been added and enabled by default.

- Add `yaml_decode` to standard library. (@mattdurham, @djcode)

- Allow override debug metrics level for `otelcol.*` components. (@hainenber)

- Add an initial lower limit of 10 seconds for the the `poll_frequency`
  argument in the `remotecfg` block. (@tpaschalis)

- Add a constant jitter to `remotecfg` service's polling. (@tpaschalis)

- Added support for NS records to `discovery.dns`. (@djcode)

- Improved clustering use cases for tracking GCP delta metrics in the `prometheus.exporter.gcp` (@kgeckhart)

- Add the `targets` argument to the `prometheus.exporter.snmp` component to support passing SNMP targets at runtime. (@wildum)

- Prefix Faro measurement values with `value_` to align with the latest Faro cloud receiver updates. (@codecapitano)

- Add `base64_decode` to standard library. (@hainenber)

- Updated OpenTelemetry Contrib to [v0.102.0](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases/tag/v0.102.0). (@mattdurham)
  - `otelcol.processor.resourcedetection`: Added a `tags` config argument to the `azure` detection mechanism.
  It exposes regex-matched Azure resource tags as OpenTelemetry resource attributes.

- A new `snmp_context` configuration argument for `prometheus.exporter.snmp`
  which overrides the `context_name` parameter in the SNMP configuration file. (@ptodev)

- Add extra configuration options for `beyla.ebpf` to select Kubernetes objects to monitor. (@marctc)

### Bugfixes

- Fixed an issue with `prometheus.scrape` in which targets that move from one
  cluster instance to another could have a staleness marker inserted and result
  in a gap in metrics (@thampiotr)

- Fix panic when `import.git` is given a revision that does not exist on the remote repo. (@hainenber)

- Fixed an issue with `loki.source.docker` where collecting logs from targets configured with multiple networks would result in errors. (@wildum)

- Fixed an issue where converting OpenTelemetry Collector configs with unused telemetry types resulted in those types being explicitly configured with an empty array in `output` blocks, rather than them being omitted entirely. (@rfratto)

### Other changes

- `pyroscope.ebpf`, `pyroscope.java`, `pyroscope.scrape`, `pyroscope.write` and `discovery.process` components are now GA. (@korniltsev)

- `prometheus.exporter.snmp`: Updating SNMP exporter from v0.24.1 to v0.26.0. (@ptodev, @erikbaranowski)

- `prometheus.scrape` component's `enable_protobuf_negotiation` argument is now
  deprecated and will be removed in a future major release.
  Use `scrape_protocols` instead and refer to `prometheus.scrape` reference
  documentation for further details. (@thampiotr)

- Updated Prometheus dependency to [v2.51.2](https://github.com/prometheus/prometheus/releases/tag/v2.51.2) (@thampiotr)

- Upgrade Beyla from v1.5.1 to v1.6.3. (@marctc)

v1.1.1
------

### Bugfixes

- Fix panic when component ID contains `/` in `otelcomponent.MustNewType(ID)`.(@qclaogui)

- Exit Alloy immediately if the port it runs on is not available.
  This port can be configured with `--server.http.listen-addr` or using
  the default listen address`127.0.0.1:12345`. (@mattdurham)

- Fix a panic in `loki.source.docker` when trying to stop a target that was never started. (@wildum)

- Fix error on boot when using IPv6 advertise addresses without explicitly
  specifying a port. (@matthewpi)

- Fix an issue where having long component labels (>63 chars) on otelcol.auth
  components lead to a panic. (@tpaschalis)

- Update `prometheus.exporter.snowflake` with the [latest](https://github.com/grafana/snowflake-prometheus-exporter) version of the exporter as of May 28, 2024 (@StefanKurek)
  - Fixes issue where returned `NULL` values from database could cause unexpected errors.

- Bubble up SSH key conversion error to facilitate failed `import.git`. (@hainenber)

v1.1.0
------

### Features

- (_Public preview_) Add support for setting GOMEMLIMIT based on cgroup setting. (@mattdurham)
- (_Experimental_) A new `otelcol.exporter.awss3` component for sending telemetry data to a S3 bucket. (@Imshelledin21)

- (_Public preview_) Introduce BoringCrypto Docker images.
  The BoringCrypto image is tagged with the `-boringcrypto` suffix and
  is only available on AMD64 and ARM64 Linux containers.
  (@rfratto, @mattdurham)

- (_Public preview_) Introduce `boringcrypto` release assets. BoringCrypto
  builds are publshed for Linux on AMD64 and ARM64 platforms. (@rfratto,
  @mattdurham)

- `otelcol.exporter.loadbalancing`: Add a new `aws_cloud_map` resolver. (@ptodev)

- Introduce a `otelcol.receiver.file_stats` component from the upstream
  OpenTelemetry `filestatsreceiver` component. (@rfratto)

### Enhancements

- Update `prometheus.exporter.kafka` with the following functionalities (@wildum):

  * GSSAPI config
  * enable/disable PA_FX_FAST
  * set a TLS server name
  * show the offset/lag for all consumer group or only the connected ones
  * set the minimum number of topics to monitor
  * enable/disable auto-creation of requested topics if they don't already exist
  * regex to exclude topics / groups
  * added metric kafka_broker_info

- In `prometheus.exporter.kafka`, the interpolation table used to compute estimated lag metrics is now pruned
  on `metadata_refresh_interval` instead of `prune_interval_seconds`. (@wildum)

- Don't restart tailers in `loki.source.kubernetes` component by above-average
  time deltas if K8s version is >= 1.29.1 (@hainenber)

- In `mimir.rules.kubernetes`, add support for running in a cluster of Alloy instances
  by electing a single instance as the leader for the `mimir.rules.kubernetes` component
  to avoid conflicts when making calls to the Mimir API. (@56quarters)

- Add the possibility of setting custom labels for the AWS Firehose logs via `X-Amz-Firehose-Common-Attributes` header. (@andriikushch)

### Bugfixes

- Fixed issue with defaults for Beyla component not being applied correctly. (marctc)

- Fix an issue on Windows where uninstalling Alloy did not remove it from the
  Add/Remove programs list. (@rfratto)

- Fixed issue where text labels displayed outside of component node's boundary. (@hainenber)

- Fix a bug where a topic was claimed by the wrong consumer type in `otelcol.receiver.kafka`. (@wildum)

- Fix an issue where nested import.git config blocks could conflict if they had the same labels. (@wildum)

- In `mimir.rules.kubernetes`, fix an issue where unrecoverable errors from the Mimir API were retried. (@56quarters)

- Fix an issue where `faro.receiver`'s `extra_log_labels` with empty value
  don't map existing value in log line. (@hainenber)

- Fix an issue where `prometheus.remote_write` only queued data for sending
  every 15 seconds instead of as soon as data was written to the WAL.
  (@rfratto)

- Imported code using `slog` logging will now not panic and replay correctly when logged before the logging
  config block is initialized. (@mattdurham)

- Fix a bug where custom components would not shadow the stdlib. If you have a module whose name conflicts with an stdlib function
  and if you use this exact function in your config, then you will need to rename your module. (@wildum)

- Fix an issue where `loki.source.docker` stops collecting logs after a container restart. (@wildum)

- Upgrading `pyroscope/ebpf` from 0.4.6 to 0.4.7 (@korniltsev):
  * detect libc version properly when libc file name is libc-2.31.so and not libc.so.6
  * treat elf files with short build id (8 bytes) properly

### Other changes

- Update `alloy-mixin` to use more specific alert group names (for example,
  `alloy_clustering` instead of `clustering`) to avoid collision with installs
  of `agent-flow-mixin`. (@rfratto)
- Upgrade Beyla from v1.4.1 to v1.5.1. (@marctc)

- Add a description to Alloy DEB and RPM packages. (@rfratto)

- Allow `pyroscope.scrape` to scrape `alloy.internal:12345`. (@hainenber)

- The latest Windows Docker image is now pushed as `nanoserver-1809` instead of
  `latest-nanoserver-1809`. The old tag will no longer be updated, and will be
  removed in a future release. (@rfratto)

- The log level of `finished node evaluation` log lines has been decreased to
  'debug'. (@tpaschalis)

- Update post-installation scripts for DEB/RPM packages to ensure
  `/var/lib/alloy` exists before configuring its permissions and ownership.
  (@rfratto)

- Remove setcap for `cap_net_bind_service` to allow alloy to run in restricted environments.
  Modern container runtimes allow binding to unprivileged ports as non-root. (@BlackDex)

- Upgrading from OpenTelemetry v0.96.0 to v0.99.0.

  - `otelcol.processor.batch`: Prevent starting unnecessary goroutines.
    https://github.com/open-telemetry/opentelemetry-collector/issues/9739
  - `otelcol.exporter.otlp`: Checks for port in the config validation for the otlpexporter.
    https://github.com/open-telemetry/opentelemetry-collector/issues/9505
  - `otelcol.receiver.otlp`: Fix bug where the otlp receiver did not properly respond
    with a retryable error code when possible for http.
    https://github.com/open-telemetry/opentelemetry-collector/pull/9357
  - `otelcol.receiver.vcenter`: Fixed the resource attribute model to more accurately support multi-cluster deployments.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30879
    For more information on impacts please refer to:
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/31113
    The main impact is that `vcenter.resource_pool.name`, `vcenter.resource_pool.inventory_path`,
    and `vcenter.cluster.name` are reported with more accuracy on VM metrics.
  - `otelcol.receiver.vcenter`: Remove the `vcenter.cluster.name` resource attribute from Host resources if the Host is standalone (no cluster).
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32548
  - `otelcol.receiver.vcenter`: Changes process for collecting VMs & VM perf metrics to be more efficient (one call now for all VMs).
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31837
  - `otelcol.connector.servicegraph`: Added a new `database_name_attribute` config argument to allow users to
    specify a custom attribute name for identifying the database name in span attributes.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/30726
  - `otelcol.connector.servicegraph`: Fix 'failed to find dimensions for key' error from race condition in metrics cleanup.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31701
  - `otelcol.connector.spanmetrics`: Add `metrics_expiration` option to enable expiration of metrics if spans are not received within a certain time frame.
    By default, the expiration is disabled (set to 0).
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30559
  - `otelcol.connector.spanmetrics`: Change default value of `metrics_flush_interval` from 15s to 60s.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31776
  - `otelcol.connector.spanmetrics`: Discard counter span metric exemplars after each flush interval to avoid unbounded memory growth.
    This aligns exemplar discarding for counter span metrics with the existing logic for histogram span metrics.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31683
  - `otelcol.exporter.loadbalancing`: Fix panic when a sub-exporter is shut down while still handling requests.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31410
  - `otelcol.exporter.loadbalancing`: Fix memory leaks on shutdown.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/31050
  - `otelcol.exporter.loadbalancing`: Support the timeout period of k8s resolver list watch can be configured.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31757
  - `otelcol.processor.transform`: Change metric unit for metrics extracted with `extract_count_metric()` to be the default unit (`1`).
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31575
  - `otelcol.receiver.opencensus`: Refactor the receiver to pass lifecycle tests and avoid leaking gRPC connections.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31643
  - `otelcol.extension.jaeger_remote_sampling`: Fix leaking goroutine on shutdown.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31157
  - `otelcol.receiver.kafka`: Fix panic on shutdown.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31926
  - `otelcol.processor.resourcedetection`: Only attempt to detect Kubernetes node resource attributes when they're enabled.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31941
  - `otelcol.processor.resourcedetection`: Fix memory leak on AKS.
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/32574
  - `otelcol.processor.resourcedetection`: Update to ec2 scraper so that core attributes are not dropped if describeTags returns an error (likely due to permissions).
    https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/30672

- Use Go 1.22.3 for builds. (@kminehart)

v1.0.0
------

### Features

- Support for programmable pipelines using a rich expression-based syntax.

- Over 130 components for processing, transforming, and exporting telemetry
  data.

- Native support for Kubernetes and Prometheus Operator without needing to
  deploy or learn a separate Kubernetes operator.

- Support for creating and sharing custom components.

- Support for forming a cluster of Alloy instances for automatic workload
  distribution.

- (_Public preview_) Support for receiving configuration from a server for
  centralized configuration management.

- A built-in UI for visualizing and debugging pipelines.

[contributors guide]: ./docs/developer/contributing.md#updating-the-changelog
