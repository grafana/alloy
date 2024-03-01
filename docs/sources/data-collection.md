---
canonical: https://grafana.com/docs/alloy/latest/data-collection/
description: Grafana Alloy data collection
menuTitle: Data collection
title: Grafana Alloy data collection
weight: 900
---

# {{% param "PRODUCT_NAME" %}} Data collection

By default, {{< param "PRODUCT_NAME" >}} sends anonymous but uniquely identifiable usage information from your {{< param "PRODUCT_NAME" >}} instance to Grafana Labs.
These statistics are sent to `stats.grafana.org`.

Statistics help us better understand how {{< param "PRODUCT_NAME" >}} is used. This helps us prioritize features and documentation.

The usage information includes the following details:

* A randomly generated, anonymous unique ID (UUID).
* Timestamp of when the UID was first generated.
* Timestamp of when the report was created (by default, every four hours).
* Version of running {{< param "PRODUCT_NAME" >}}.
* Operating system {{< param "PRODUCT_NAME" >}} is running on.
* System architecture {{< param "PRODUCT_NAME" >}} is running on.
* List of enabled [components][]
* Method used to deploy {{< param "PRODUCT_NAME" >}}, for example Docker, Helm, RPM, or Operator.

This list may change over time. All newly reported data is documented in the CHANGELOG.

## Opt-out of data collection

You can use the `-disable-reporting` [command line flag][] to disable the reporting and opt-out of the data collection.

[components]: ../concepts/components
[command line flag]: ../reference/cli/run
