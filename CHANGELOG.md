# Changelog

> _Contributors should read our [contributors guide][] for instructions on how
> to update the changelog._

This document contains a historical list of changes between releases. Only
changes that impact end-user behavior are listed; changes to documentation or
internal API changes are not present.

Main (unreleased)
-----------------

### Features

- (_Public preview_) Add support for setting GOMEMLIMIT based on cgroup setting. (@mattdurham)

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

- Add support for configuring CPU profile's duration scraped by `pyroscope.scrape`. (@hainenber)

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

v1.0.0 (2024-04-09)
-------------------

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
