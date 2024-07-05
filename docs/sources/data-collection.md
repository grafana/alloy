---
canonical: https://grafana.com/docs/alloy/latest/data-collection/
description: Grafana Alloy data collection
menuTitle: Data collection
title: Grafana Alloy data collection
weight: 900
---

# {{% param "FULL_PRODUCT_NAME" %}} data collection

By default, {{< param "FULL_PRODUCT_NAME" >}} sends anonymous but uniquely identifiable usage information from your {{< param "PRODUCT_NAME" >}} instance to Grafana Labs.
These statistics are sent to `stats.grafana.org`.

Statistics help Grafana better understand how {{< param "PRODUCT_NAME" >}} is used. This helps us prioritize features and documentation.

The usage information includes the following details:

* A randomly generated, anonymous, unique ID (UUID).
* Timestamp of when the UID was first generated.
* Timestamp of when the report was created (by default, every four hours).
* The version of {{< param "PRODUCT_NAME" >}}.
* The operating system {{< param "PRODUCT_NAME" >}} is running on.
* The system architecture {{< param "PRODUCT_NAME" >}} is running on.
* A list of enabled [components][]
* The method used to deploy {{< param "PRODUCT_NAME" >}}, for example Docker, Helm, or a Linux package.

This list may change over time.
All newly reported data is documented in the CHANGELOG.

## Opt-out of data collection

You can use the `-disable-reporting` [command line flag][] to disable the reporting and opt-out of the data collection.

[components]: ../get-started/components
[command line flag]: ../reference/cli/run
