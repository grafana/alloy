---
aliases:
- /docs/grafana-cloud/agent/flow/reference/cli/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/cli/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/cli/
- /docs/grafana-cloud/send-data/agent/flow/reference/cli/
canonical: https://grafana.com/docs/agent/latest/flow/reference/cli/
description: Learn about the Grafana Agent command line interface
menuTitle: Command-line interface
title: The Grafana Agent command-line interface
weight: 100
---

# The {{% param "PRODUCT_ROOT_NAME" %}} command-line interface

When in Flow mode, the `grafana-agent` binary exposes a command-line interface with
subcommands to perform various operations.

The most common subcommand is [`run`][run] which accepts a configuration file and
starts {{< param "PRODUCT_NAME" >}}.

Available commands:

* [`convert`][convert]: Convert a {{< param "PRODUCT_ROOT_NAME" >}} configuration file.
* [`fmt`][fmt]: Format a {{< param "PRODUCT_NAME" >}} configuration file.
* [`run`][run]: Start {{< param "PRODUCT_NAME" >}}, given a configuration file.
* [`tools`][tools]: Read the WAL and provide statistical information.
* `completion`: Generate shell completion for the `grafana-agent-flow` CLI.
* `help`: Print help for supported commands.

[run]: {{< relref "./run.md" >}}
[fmt]: {{< relref "./fmt.md" >}}
[convert]: {{< relref "./convert.md" >}}
[tools]: {{< relref "./tools.md" >}}
