---
canonical: https://grafana.com/docs/alloy/latest/reference/components/kafka/
description: Learn about the kafka components in Grafana Alloy
labels:
  products:
    - oss
title: kafka
weight: 100
---

# `kafka`

The `kafka` components buffer telemetry for many tenants in a single Kafka topic, where each tenant owns exactly one partition.
Use `kafka.tenant_producer` to write incoming requests to Kafka, and `kafka.tenant_consumer` to read them back into {{< param "PRODUCT_NAME" >}} pipelines while keeping each tenant's data separate.

{{< section >}}
