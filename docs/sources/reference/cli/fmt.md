---
aliases:
- ./reference/cli/fmt/
canonical: https://grafana.com/docs/alloy/latest/reference/cli/fmt/
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

If the `FILE_NAME` argument isn't provided or if the `FILE_NAME` argument is equal to `-`, `fmt` formats the contents of standard input.
Otherwise, `fmt` reads and formats the file from disk specified by the argument.

The `--write` flag can be specified to replace the contents of the original file on disk with the formatted results.
`--write` can only be provided when `fmt` isn't reading from standard input.

The command fails if the file being formatted has syntactically incorrect River configuration, but doesn't validate whether {{< param "PRODUCT_NAME" >}} components are configured properly.

The following flags are supported:

* `--write`, `-w`: Write the formatted file back to disk when not reading from standard input.
