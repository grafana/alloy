# Changelog

> _Contributors should read our [contributors guide][] for instructions on how
> to update the changelog._

This document contains a historical list of changes between releases. Only
changes that impact end-user behavior are listed; changes to documentation or
internal API changes are not present.

Main (unreleased)
-----------------

### Features

- Add the possibility to export span events as logs in `otelcol.connector.spanlogs`. (@steve-hb)

### Enhancements

- (_Experimental_) Log instance label key in `database_observability.mysql` (@cristiangreco)

- (_Experimental_) Improve parsing of truncated queries in `database_observability.mysql` (@cristiangreco)

- (_Experimental_) Capture schema name for query samples in `database_observability.mysql` (@cristiangreco)

- (_Experimental_) Fix handling of view table types when detecting schema in `database_observability.mysql` (@matthewnolf)

- (_Experimental_) fix error handling during result set iteration in `database_observability.mysql` (@cristiangreco)

- Add json format support for log export via faro receiver (@ravishankar15)

### Bugfixes

- Fix log rotation for Windows in `loki.source.file` by refactoring the component to use the runner pkg. This should also reduce CPU consumption when tailing a lot of files in a dynamic environment. (@wildum)

- Add livedebugging support for `prometheus.remote_write` (@ravishankar15)

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
