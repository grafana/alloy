---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/
description: Learn about the Grafana Alloy command line interface
menuTitle: Command-line interface
title: The Grafana Alloy command-line interface
weight: 50
---

# The {{% param "FULL_PRODUCT_NAME" %}} command-line interface

The `alloy` binary exposes a command-line interface with subcommands to perform various operations.

The most common subcommand is [`run`][run] which accepts a configuration file and starts {{< param "PRODUCT_NAME" >}}.

Available commands:

* [`convert`][convert]: Convert an {{< param "PRODUCT_NAME" >}} configuration file.
* [`fmt`][fmt]: Format an {{< param "PRODUCT_NAME" >}} configuration file.
* [`run`][run]: Start {{< param "PRODUCT_NAME" >}}, given a configuration file.
* [`tools`][tools]: Read the WAL and provide statistical information.
* `completion`: Generate shell completion for the `alloy` CLI.
* `help`: Print help for supported commands.

[run]: ./run/
[fmt]: ./fmt/
[convert]: ./convert/
[tools]: ./tools/
