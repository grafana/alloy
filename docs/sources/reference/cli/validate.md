---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/validate/
description: Learn about the validate command
menuTitle: validate
title: The validate command
weight: 200
---

# The `validate` command

The `validate` command validates {{< param "PRODUCT_NAME" >}} configuration file/directory path.

## Usage

```shell
alloy validate [<FLAG> ...] <PATH_NAME>
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the input and output of the command.
* _`<PATH_NAME>`_: Required. The {{< param "PRODUCT_NAME" >}} configuration file/directory path.

If you give the _`<PATH_NAME>`_ argument a directory path, {{< param "PRODUCT_NAME" >}} finds `*.alloy` files (ignoring nested directories) and loads them as a single configuration source.

The following flags are supported:

* `--config.format`: The format of the source file. Supported formats: `alloy`, `otelcol`, `prometheus`, `promtail`, `static` (default `"alloy"`).
* `--config.bypass-conversion-errors`: Enable bypassing errors when converting (default `false`).
* `--config.extra-args`: Extra arguments from the original format used by the converter.
* `--stability.level`: The minimum permitted stability level of functionality. Supported values: `experimental`, `public-preview`, `generally-available` (default `"generally-available"`).
* `--feature.community-components.enabled`: Enable community components (default `false`).

## Limitations

Validation performed is limited in what it can check. Currently it will check:

* Syntax errors.
* Components exist.
* Components are **unique** across all {{< param "PRODUCT_NAME" >}} configuration file and are not repeated.
