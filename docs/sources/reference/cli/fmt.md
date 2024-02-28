---
aliases:
- /docs/grafana-cloud/agent/flow/reference/cli/fmt/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/cli/fmt/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/cli/fmt/
- /docs/grafana-cloud/send-data/agent/flow/reference/cli/fmt/
canonical: https://grafana.com/docs/agent/latest/flow/reference/cli/fmt/
description: Learn about the fmt command
menuTitle: fmt
title: The fmt command
weight: 200
---

# The fmt command

The `fmt` command formats a given {{< param "PRODUCT_NAME" >}} configuration file.

## Usage

Usage:

* `AGENT_MODE=flow grafana-agent fmt [FLAG ...] FILE_NAME`
* `grafana-agent-flow fmt [FLAG ...] FILE_NAME`

   Replace the following:

   * `FLAG`: One or more flags that define the input and output of the command.
   * `FILE_NAME`: The {{< param "PRODUCT_NAME" >}} configuration file.

If the `FILE_NAME` argument is not provided or if the `FILE_NAME` argument is
equal to `-`, `fmt` formats the contents of standard input. Otherwise,
`fmt` reads and formats the file from disk specified by the argument.

The `--write` flag can be specified to replace the contents of the original
file on disk with the formatted results. `--write` can only be provided when
`fmt` is not reading from standard input.

The command fails if the file being formatted has syntactically incorrect River
configuration, but does not validate whether Flow components are configured
properly.

The following flags are supported:

* `--write`, `-w`: Write the formatted file back to disk when not reading from
  standard input.
