---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/validate/
description: Learn about the validate command
labels:
  stage: general-availability
  products:
    - oss
title: validate
weight: 500
---

# `validate`

The `validate` command validates an {{< param "PRODUCT_NAME" >}} configuration file or directory path.

## Usage

```shell
alloy validate [<FLAG> ...] <PATH_NAME>
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the input and output of the command.
* _`<PATH_NAME>`_: Required. The {{< param "PRODUCT_NAME" >}} configuration file or directory path.

If the configuration file is valid, the `validate` command returns a zero exit code.
If the configuration file is invalid, the command  returns a non-zero exit code and prints diagnostics generated during validation to stderr.

If you provide a directory path for  the _`<PATH_NAME>`_, {{< param "PRODUCT_NAME" >}} finds `*.alloy` files, ignoring nested directories, and loads them as a single configuration source.

The following flags are supported:

* `--config.format`: Specifies the source file format. Supported formats: `alloy`, `otelcol`, `prometheus`, `promtail`, and `static` (default `"alloy"`).
* `--config.bypass-conversion-errors`: Enable bypassing errors during conversion (default `false`).
* `--config.extra-args`: Extra arguments from the original format used by the converter.
* `--stability.level`: The minimum permitted stability level of functionality. Supported values: `experimental`, `public-preview`, and `generally-available` (default `"generally-available"`).
* `--feature.community-components.enabled`: Enable community components (default `false`).

{{< admonition type="note" >}}
When you validate the {{< param "PRODUCT_NAME" >}} configuration, you must set the `--stability.level` and `--feature.community-components.enabled` arguments to the same values you want to use when you run {{< param "PRODUCT_NAME" >}}.
{{< /admonition >}}

## Limitations

Validation is limited in scope. It currently checks for:

* Syntax errors.
* Missing components.
* Component name conflicts.
* Required properties are set.
* Unknown properties.
* Foreach blocks.
* Declare blocks.
