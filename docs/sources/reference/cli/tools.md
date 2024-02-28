---
aliases:
- /docs/grafana-cloud/agent/flow/reference/cli/tools/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/cli/tools/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/cli/tools/
- /docs/grafana-cloud/send-data/agent/flow/reference/cli/tools/
canonical: https://grafana.com/docs/agent/latest/flow/reference/cli/tools/
description: Learn about the tools command
menuTitle: tools
title: The tools command
weight: 400
---

# The tools command

The `tools` command contains command line tooling grouped by Flow component.

{{< admonition type="caution" >}}
Utilities in this command have no backward compatibility
guarantees and may change or be removed between releases.
{{< /admonition >}}

## Subcommands

### prometheus.remote_write sample-stats

Usage:

* `AGENT_MODE=flow grafana-agent tools prometheus.remote_write sample-stats [FLAG ...] WAL_DIRECTORY`
* `grafana-agent-flow tools prometheus.remote_write sample-stats [FLAG ...] WAL_DIRECTORY`

The `sample-stats` command reads the Write-Ahead Log (WAL) specified by
`WAL_DIRECTORY` and collects information on metric samples within it.

For each metric discovered, `sample-stats` emits:

* The timestamp of the oldest sample received for that metric.
* The timestamp of the newest sample received for that metric.
* The total number of samples discovered for that metric.

By default, `sample-stats` will return information for every metric in the WAL.
You can pass the `--selector` flag to filter the reported metrics to a smaller set.

The following flag is supported:

* `--selector`: A PromQL label selector to filter data by. (default `{}`)

### prometheus.remote_write target-stats

Usage:

* `AGENT_MODE=flow grafana-agent tools prometheus.remote_write target-stats --job JOB --instance INSTANCE WAL_DIRECTORY`
* `grafana-agent-flow tools prometheus.remote_write target-stats --job JOB --instance INSTANCE WAL_DIRECTORY`

The `target-stats` command reads the Write-Ahead Log (WAL) specified by
`WAL_DIRECTORY` and collects metric cardinality information for a specific
target.

For the target specified by the `--job` and `--instance` flags, unique metric
names for that target are printed along with the number of series with that
metric name.

The following flags are supported:

* `--job`: The `job` label of the target.
* `--instance`: The `instance` label of the target.

The `--job` and `--instance` labels are required.

### prometheus.remote_write wal-stats

Usage: 

* `AGENT_MODE=flow grafana-agent tools prometheus.remote_write wal-stats WAL_DIRECTORY`
* `grafana-agent-flow tools prometheus.remote_write wal-stats WAL_DIRECTORY`

The `wal-stats` command reads the Write-Ahead Log (WAL) specified by
`WAL_DIRECTORY` and collects general information about it.

The following information is reported:

* The timestamp of the oldest sample in the WAL.
* The timestamp of the newest sample in the WAL.
* The total number of unique series defined in the WAL.
* The total number of samples in the WAL.
* The number of hash collisions detected, if any.
* The total number of invalid records in the WAL, if any.
* The most recent WAL checkpoint segment number.
* The oldest segment number in the WAL.
* The newest segment number in the WAL.

Additionally, `wal-stats` reports per-target information, where a target is
defined as a unique combination of the `job` and `instance` label values. For
each target, `wal-stats` reports the number of series and the number of
metric samples associated with that target.

The `wal-stats` command does not support any flags.
