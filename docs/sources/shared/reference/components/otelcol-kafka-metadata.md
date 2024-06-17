---
description: Shared content, otelcol Kafka metadata
headless: true
---

The `metadata` block configures how to retrieve and store metadata from the
Kafka broker.

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`include_all_topics` | `bool` | When true, maintains metadata for all topics. | `true` | no

If the `include_all_topics` argument is `true`, 
a full set of metadata for all topics is maintained rather than the minimal set
that has been necessary so far. Including the full set of metadata is more
convenient for users but can consume a substantial amount of memory if you have
many topics and partitions.

Retrieving metadata may fail if the Kafka broker is starting up at the same
time as the Alloy component. The `retry` child block can be provided to customize retry behavior.
