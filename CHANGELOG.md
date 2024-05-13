# Changelog

> _Contributors should read our [contributors guide][] for instructions on how
> to update the changelog._

This document contains a historical list of changes between releases. Only
changes that impact end-user behavior are listed; changes to documentation or
internal API changes are not present.

Main (unreleased)
-----------------

### Breaking changes to non-GA functionality

- Update Public preview `remotecfg` to use `alloy-remote-config` instead of `agent-remote-config`. The
  API has been updated to use the term `collector` over `agent`. (@erikbaranowski)

### Enhancements

- Add native histogram support to `otelcol.receiver.prometheus`. (@wildum)

### Bugfixes

- Fixed an issue with `prometheus.scrape` in which targets that move from one
  cluster instance to another could have a staleness marker inserted and result
  in a gap in metrics (@thampiotr)

- Exit Alloy immediately if the port it runs on is not available. 
  This port can be configured with `--server.http.listen-addr` or using
  the default listen address`127.0.0.1:12345`. (@mattdurham) 

### Other changes

- `prometheus.exporter.snmp`: Updating SNMP exporter from v0.24.1 to v0.26.0.

v1.1.0-rc.0
-----------

### Features

- (_Public preview_) Add support for setting GOMEMLIMIT based on cgroup setting. (@mattdurham)

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
