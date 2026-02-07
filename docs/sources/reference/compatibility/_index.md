---
canonical: https://grafana.com/docs/alloy/latest/reference/compatibility/
description: Learn about which components are compatible with each other in Grafana Alloy
title: Compatible components
weight: 400
---

# Compatible components

This section provides an overview of _some_ of the possible connections between compatible components in {{< param "PRODUCT_NAME" >}}.

For each common data type, we provide a list of compatible components that can export or consume it.

{{< admonition type="note" >}}
The type of export may not be the only requirement for chaining components together.
The value of an attribute may matter as well as its type.
Refer to each component's documentation for more details on what values are acceptable.

For example:

* A Prometheus component may always expect an `"__address__"` label inside a list of targets.
* A `string` argument may only accept certain values like "traceID" or "spanID".
{{< /admonition >}}

## Targets

Targets are a `list(map(string))` - a [list][] of [maps][] with [string][] values.
They can contain different key-value pairs, and you can use them with a wide range of components.
Some components require Targets to contain specific key-value pairs to work correctly.
It's recommended to always check component references for details when working with Targets.

[list]: ../../get-started/configuration-syntax/expressions/types_and_values/#naming-convention
[maps]: ../../get-started/configuration-syntax/expressions/types_and_values/#naming-convention
[string]: ../../get-started/configuration-syntax/expressions/types_and_values/#strings

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Targets Exporters

The following components, grouped by namespace, _export_ Targets.

<!-- START GENERATED SECTION: EXPORTERS OF Targets -->

{{< collapse title="beyla" >}}
- [beyla.ebpf](../components/beyla/beyla.ebpf)
{{< /collapse >}}

{{< collapse title="database_observability" >}}
- [database_observability.mysql](../components/database_observability/database_observability.mysql)
- [database_observability.postgres](../components/database_observability/database_observability.postgres)
{{< /collapse >}}

{{< collapse title="discovery" >}}
- [discovery.azure](../components/discovery/discovery.azure)
- [discovery.consul](../components/discovery/discovery.consul)
- [discovery.consulagent](../components/discovery/discovery.consulagent)
- [discovery.digitalocean](../components/discovery/discovery.digitalocean)
- [discovery.dns](../components/discovery/discovery.dns)
- [discovery.docker](../components/discovery/discovery.docker)
- [discovery.dockerswarm](../components/discovery/discovery.dockerswarm)
- [discovery.ec2](../components/discovery/discovery.ec2)
- [discovery.eureka](../components/discovery/discovery.eureka)
- [discovery.file](../components/discovery/discovery.file)
- [discovery.gce](../components/discovery/discovery.gce)
- [discovery.hetzner](../components/discovery/discovery.hetzner)
- [discovery.http](../components/discovery/discovery.http)
- [discovery.ionos](../components/discovery/discovery.ionos)
- [discovery.kubelet](../components/discovery/discovery.kubelet)
- [discovery.kubernetes](../components/discovery/discovery.kubernetes)
- [discovery.kuma](../components/discovery/discovery.kuma)
- [discovery.lightsail](../components/discovery/discovery.lightsail)
- [discovery.linode](../components/discovery/discovery.linode)
- [discovery.marathon](../components/discovery/discovery.marathon)
- [discovery.nerve](../components/discovery/discovery.nerve)
- [discovery.nomad](../components/discovery/discovery.nomad)
- [discovery.openstack](../components/discovery/discovery.openstack)
- [discovery.ovhcloud](../components/discovery/discovery.ovhcloud)
- [discovery.process](../components/discovery/discovery.process)
- [discovery.puppetdb](../components/discovery/discovery.puppetdb)
- [discovery.relabel](../components/discovery/discovery.relabel)
- [discovery.scaleway](../components/discovery/discovery.scaleway)
- [discovery.serverset](../components/discovery/discovery.serverset)
- [discovery.triton](../components/discovery/discovery.triton)
- [discovery.uyuni](../components/discovery/discovery.uyuni)
{{< /collapse >}}

{{< collapse title="local" >}}
- [local.file_match](../components/local/local.file_match)
{{< /collapse >}}

{{< collapse title="prometheus" >}}
- [prometheus.exporter.apache](../components/prometheus/prometheus.exporter.apache)
- [prometheus.exporter.azure](../components/prometheus/prometheus.exporter.azure)
- [prometheus.exporter.blackbox](../components/prometheus/prometheus.exporter.blackbox)
- [prometheus.exporter.cadvisor](../components/prometheus/prometheus.exporter.cadvisor)
- [prometheus.exporter.catchpoint](../components/prometheus/prometheus.exporter.catchpoint)
- [prometheus.exporter.cloudwatch](../components/prometheus/prometheus.exporter.cloudwatch)
- [prometheus.exporter.consul](../components/prometheus/prometheus.exporter.consul)
- [prometheus.exporter.databricks](../components/prometheus/prometheus.exporter.databricks)
- [prometheus.exporter.dnsmasq](../components/prometheus/prometheus.exporter.dnsmasq)
- [prometheus.exporter.elasticsearch](../components/prometheus/prometheus.exporter.elasticsearch)
- [prometheus.exporter.gcp](../components/prometheus/prometheus.exporter.gcp)
- [prometheus.exporter.github](../components/prometheus/prometheus.exporter.github)
- [prometheus.exporter.kafka](../components/prometheus/prometheus.exporter.kafka)
- [prometheus.exporter.memcached](../components/prometheus/prometheus.exporter.memcached)
- [prometheus.exporter.mongodb](../components/prometheus/prometheus.exporter.mongodb)
- [prometheus.exporter.mssql](../components/prometheus/prometheus.exporter.mssql)
- [prometheus.exporter.mysql](../components/prometheus/prometheus.exporter.mysql)
- [prometheus.exporter.oracledb](../components/prometheus/prometheus.exporter.oracledb)
- [prometheus.exporter.postgres](../components/prometheus/prometheus.exporter.postgres)
- [prometheus.exporter.process](../components/prometheus/prometheus.exporter.process)
- [prometheus.exporter.redis](../components/prometheus/prometheus.exporter.redis)
- [prometheus.exporter.self](../components/prometheus/prometheus.exporter.self)
- [prometheus.exporter.snmp](../components/prometheus/prometheus.exporter.snmp)
- [prometheus.exporter.snowflake](../components/prometheus/prometheus.exporter.snowflake)
- [prometheus.exporter.squid](../components/prometheus/prometheus.exporter.squid)
- [prometheus.exporter.static](../components/prometheus/prometheus.exporter.static)
- [prometheus.exporter.statsd](../components/prometheus/prometheus.exporter.statsd)
- [prometheus.exporter.unix](../components/prometheus/prometheus.exporter.unix)
- [prometheus.exporter.windows](../components/prometheus/prometheus.exporter.windows)
{{< /collapse >}}

<!-- END GENERATED SECTION: EXPORTERS OF Targets -->

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Targets Consumers

The following components, grouped by namespace, _consume_ Targets.

<!-- START GENERATED SECTION: CONSUMERS OF Targets -->

{{< collapse title="database_observability" >}}
- [database_observability.mysql](../components/database_observability/database_observability.mysql)
- [database_observability.postgres](../components/database_observability/database_observability.postgres)
{{< /collapse >}}

{{< collapse title="discovery" >}}
- [discovery.process](../components/discovery/discovery.process)
- [discovery.relabel](../components/discovery/discovery.relabel)
{{< /collapse >}}

{{< collapse title="local" >}}
- [local.file_match](../components/local/local.file_match)
{{< /collapse >}}

{{< collapse title="loki" >}}
- [loki.enrich](../components/loki/loki.enrich)
- [loki.source.docker](../components/loki/loki.source.docker)
- [loki.source.file](../components/loki/loki.source.file)
- [loki.source.kubernetes](../components/loki/loki.source.kubernetes)
{{< /collapse >}}

{{< collapse title="otelcol" >}}
- [otelcol.processor.discovery](../components/otelcol/otelcol.processor.discovery)
{{< /collapse >}}

{{< collapse title="prometheus" >}}
- [prometheus.enrich](../components/prometheus/prometheus.enrich)
- [prometheus.scrape](../components/prometheus/prometheus.scrape)
{{< /collapse >}}

{{< collapse title="pyroscope" >}}
- [pyroscope.ebpf](../components/pyroscope/pyroscope.ebpf)
- [pyroscope.enrich](../components/pyroscope/pyroscope.enrich)
- [pyroscope.java](../components/pyroscope/pyroscope.java)
- [pyroscope.scrape](../components/pyroscope/pyroscope.scrape)
{{< /collapse >}}

<!-- END GENERATED SECTION: CONSUMERS OF Targets -->

## Prometheus `MetricsReceiver`

The Prometheus metrics are sent between components using `MetricsReceiver`s.
`MetricsReceiver`s are [capsules][] that are exported by components that can receive Prometheus metrics.
Components that can consume Prometheus metrics can be passed the `MetricsReceiver` as an argument.
Use the following components to build your Prometheus metrics pipeline:

[capsules]: ../../get-started/configuration-syntax/expressions/types_and_values/#capsules

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Prometheus `MetricsReceiver` Exporters

The following components, grouped by namespace, _export_ Prometheus `MetricsReceiver`.

<!-- START GENERATED SECTION: EXPORTERS OF Prometheus `MetricsReceiver` -->

{{< collapse title="otelcol" >}}
- [otelcol.receiver.prometheus](../components/otelcol/otelcol.receiver.prometheus)
{{< /collapse >}}

{{< collapse title="prometheus" >}}
- [prometheus.echo](../components/prometheus/prometheus.echo)
- [prometheus.enrich](../components/prometheus/prometheus.enrich)
- [prometheus.relabel](../components/prometheus/prometheus.relabel)
- [prometheus.remote_write](../components/prometheus/prometheus.remote_write)
- [prometheus.write.queue](../components/prometheus/prometheus.write.queue)
{{< /collapse >}}

<!-- END GENERATED SECTION: EXPORTERS OF Prometheus `MetricsReceiver` -->

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Prometheus `MetricsReceiver` Consumers

The following components, grouped by namespace, _consume_ Prometheus `MetricsReceiver`.

<!-- START GENERATED SECTION: CONSUMERS OF Prometheus `MetricsReceiver` -->

{{< collapse title="otelcol" >}}
- [otelcol.exporter.prometheus](../components/otelcol/otelcol.exporter.prometheus)
{{< /collapse >}}

{{< collapse title="prometheus" >}}
- [prometheus.enrich](../components/prometheus/prometheus.enrich)
- [prometheus.operator.podmonitors](../components/prometheus/prometheus.operator.podmonitors)
- [prometheus.operator.probes](../components/prometheus/prometheus.operator.probes)
- [prometheus.operator.scrapeconfigs](../components/prometheus/prometheus.operator.scrapeconfigs)
- [prometheus.operator.servicemonitors](../components/prometheus/prometheus.operator.servicemonitors)
- [prometheus.receive_http](../components/prometheus/prometheus.receive_http)
- [prometheus.relabel](../components/prometheus/prometheus.relabel)
- [prometheus.scrape](../components/prometheus/prometheus.scrape)
{{< /collapse >}}

<!-- END GENERATED SECTION: CONSUMERS OF Prometheus `MetricsReceiver` -->

## Loki `LogsReceiver`

`LogsReceiver` is a [capsule][capsules] that is exported by components that can receive Loki logs.
Components that consume `LogsReceiver` as an argument typically send logs to it.
Use the following components to build your Loki logs pipeline:

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Loki `LogsReceiver` Exporters

The following components, grouped by namespace, _export_ Loki `LogsReceiver`.

<!-- START GENERATED SECTION: EXPORTERS OF Loki `LogsReceiver` -->

{{< collapse title="loki" >}}
- [loki.echo](../components/loki/loki.echo)
- [loki.enrich](../components/loki/loki.enrich)
- [loki.process](../components/loki/loki.process)
- [loki.relabel](../components/loki/loki.relabel)
- [loki.secretfilter](../components/loki/loki.secretfilter)
- [loki.write](../components/loki/loki.write)
{{< /collapse >}}

{{< collapse title="otelcol" >}}
- [otelcol.receiver.loki](../components/otelcol/otelcol.receiver.loki)
{{< /collapse >}}

<!-- END GENERATED SECTION: EXPORTERS OF Loki `LogsReceiver` -->

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Loki `LogsReceiver` Consumers

The following components, grouped by namespace, _consume_ Loki `LogsReceiver`.

<!-- START GENERATED SECTION: CONSUMERS OF Loki `LogsReceiver` -->

{{< collapse title="database_observability" >}}
- [database_observability.mysql](../components/database_observability/database_observability.mysql)
- [database_observability.postgres](../components/database_observability/database_observability.postgres)
{{< /collapse >}}

{{< collapse title="faro" >}}
- [faro.receiver](../components/faro/faro.receiver)
{{< /collapse >}}

{{< collapse title="loki" >}}
- [loki.enrich](../components/loki/loki.enrich)
- [loki.process](../components/loki/loki.process)
- [loki.relabel](../components/loki/loki.relabel)
- [loki.secretfilter](../components/loki/loki.secretfilter)
- [loki.source.api](../components/loki/loki.source.api)
- [loki.source.awsfirehose](../components/loki/loki.source.awsfirehose)
- [loki.source.azure_event_hubs](../components/loki/loki.source.azure_event_hubs)
- [loki.source.cloudflare](../components/loki/loki.source.cloudflare)
- [loki.source.docker](../components/loki/loki.source.docker)
- [loki.source.file](../components/loki/loki.source.file)
- [loki.source.gcplog](../components/loki/loki.source.gcplog)
- [loki.source.gelf](../components/loki/loki.source.gelf)
- [loki.source.heroku](../components/loki/loki.source.heroku)
- [loki.source.journal](../components/loki/loki.source.journal)
- [loki.source.kafka](../components/loki/loki.source.kafka)
- [loki.source.kubernetes](../components/loki/loki.source.kubernetes)
- [loki.source.kubernetes_events](../components/loki/loki.source.kubernetes_events)
- [loki.source.podlogs](../components/loki/loki.source.podlogs)
- [loki.source.syslog](../components/loki/loki.source.syslog)
- [loki.source.windowsevent](../components/loki/loki.source.windowsevent)
{{< /collapse >}}

{{< collapse title="otelcol" >}}
- [otelcol.exporter.loki](../components/otelcol/otelcol.exporter.loki)
{{< /collapse >}}

<!-- END GENERATED SECTION: CONSUMERS OF Loki `LogsReceiver` -->

## OpenTelemetry `otelcol.Consumer`

The OpenTelemetry data is sent between components using `otelcol.Consumer`s.
`otelcol.Consumer`s are [capsules][] that are exported by components that can receive OpenTelemetry data.
Components that can consume OpenTelemetry data can be passed the `otelcol.Consumer` as an argument.
Some components that use `otelcol.Consumer` only support a subset of telemetry signals, for example, only traces.
Refer to the component reference pages for more details on what is supported.
Use the following components to build your OpenTelemetry pipeline:

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### OpenTelemetry `otelcol.Consumer` Exporters

The following components, grouped by namespace, _export_ OpenTelemetry `otelcol.Consumer`.

<!-- START GENERATED SECTION: EXPORTERS OF OpenTelemetry `otelcol.Consumer` -->

{{< collapse title="otelcol" >}}
- [otelcol.connector.count](../components/otelcol/otelcol.connector.count)
- [otelcol.connector.host_info](../components/otelcol/otelcol.connector.host_info)
- [otelcol.connector.servicegraph](../components/otelcol/otelcol.connector.servicegraph)
- [otelcol.connector.spanlogs](../components/otelcol/otelcol.connector.spanlogs)
- [otelcol.connector.spanmetrics](../components/otelcol/otelcol.connector.spanmetrics)
- [otelcol.exporter.awss3](../components/otelcol/otelcol.exporter.awss3)
- [otelcol.exporter.datadog](../components/otelcol/otelcol.exporter.datadog)
- [otelcol.exporter.debug](../components/otelcol/otelcol.exporter.debug)
- [otelcol.exporter.faro](../components/otelcol/otelcol.exporter.faro)
- [otelcol.exporter.file](../components/otelcol/otelcol.exporter.file)
- [otelcol.exporter.googlecloud](../components/otelcol/otelcol.exporter.googlecloud)
- [otelcol.exporter.googlecloudpubsub](../components/otelcol/otelcol.exporter.googlecloudpubsub)
- [otelcol.exporter.kafka](../components/otelcol/otelcol.exporter.kafka)
- [otelcol.exporter.loadbalancing](../components/otelcol/otelcol.exporter.loadbalancing)
- [otelcol.exporter.loki](../components/otelcol/otelcol.exporter.loki)
- [otelcol.exporter.otlp](../components/otelcol/otelcol.exporter.otlp)
- [otelcol.exporter.otlphttp](../components/otelcol/otelcol.exporter.otlphttp)
- [otelcol.exporter.prometheus](../components/otelcol/otelcol.exporter.prometheus)
- [otelcol.exporter.splunkhec](../components/otelcol/otelcol.exporter.splunkhec)
- [otelcol.exporter.syslog](../components/otelcol/otelcol.exporter.syslog)
- [otelcol.processor.attributes](../components/otelcol/otelcol.processor.attributes)
- [otelcol.processor.batch](../components/otelcol/otelcol.processor.batch)
- [otelcol.processor.cumulativetodelta](../components/otelcol/otelcol.processor.cumulativetodelta)
- [otelcol.processor.deltatocumulative](../components/otelcol/otelcol.processor.deltatocumulative)
- [otelcol.processor.discovery](../components/otelcol/otelcol.processor.discovery)
- [otelcol.processor.filter](../components/otelcol/otelcol.processor.filter)
- [otelcol.processor.groupbyattrs](../components/otelcol/otelcol.processor.groupbyattrs)
- [otelcol.processor.interval](../components/otelcol/otelcol.processor.interval)
- [otelcol.processor.k8sattributes](../components/otelcol/otelcol.processor.k8sattributes)
- [otelcol.processor.memory_limiter](../components/otelcol/otelcol.processor.memory_limiter)
- [otelcol.processor.metric_start_time](../components/otelcol/otelcol.processor.metric_start_time)
- [otelcol.processor.probabilistic_sampler](../components/otelcol/otelcol.processor.probabilistic_sampler)
- [otelcol.processor.resourcedetection](../components/otelcol/otelcol.processor.resourcedetection)
- [otelcol.processor.span](../components/otelcol/otelcol.processor.span)
- [otelcol.processor.tail_sampling](../components/otelcol/otelcol.processor.tail_sampling)
- [otelcol.processor.transform](../components/otelcol/otelcol.processor.transform)
- [otelcol.receiver.googlecloudpubsub](../components/otelcol/otelcol.receiver.googlecloudpubsub)
{{< /collapse >}}

<!-- END GENERATED SECTION: EXPORTERS OF OpenTelemetry `otelcol.Consumer` -->

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### OpenTelemetry `otelcol.Consumer` Consumers

The following components, grouped by namespace, _consume_ OpenTelemetry `otelcol.Consumer`.

<!-- START GENERATED SECTION: CONSUMERS OF OpenTelemetry `otelcol.Consumer` -->

{{< collapse title="beyla" >}}
- [beyla.ebpf](../components/beyla/beyla.ebpf)
{{< /collapse >}}

{{< collapse title="faro" >}}
- [faro.receiver](../components/faro/faro.receiver)
{{< /collapse >}}

{{< collapse title="otelcol" >}}
- [otelcol.connector.count](../components/otelcol/otelcol.connector.count)
- [otelcol.connector.host_info](../components/otelcol/otelcol.connector.host_info)
- [otelcol.connector.servicegraph](../components/otelcol/otelcol.connector.servicegraph)
- [otelcol.connector.spanlogs](../components/otelcol/otelcol.connector.spanlogs)
- [otelcol.connector.spanmetrics](../components/otelcol/otelcol.connector.spanmetrics)
- [otelcol.processor.attributes](../components/otelcol/otelcol.processor.attributes)
- [otelcol.processor.batch](../components/otelcol/otelcol.processor.batch)
- [otelcol.processor.cumulativetodelta](../components/otelcol/otelcol.processor.cumulativetodelta)
- [otelcol.processor.deltatocumulative](../components/otelcol/otelcol.processor.deltatocumulative)
- [otelcol.processor.discovery](../components/otelcol/otelcol.processor.discovery)
- [otelcol.processor.filter](../components/otelcol/otelcol.processor.filter)
- [otelcol.processor.groupbyattrs](../components/otelcol/otelcol.processor.groupbyattrs)
- [otelcol.processor.interval](../components/otelcol/otelcol.processor.interval)
- [otelcol.processor.k8sattributes](../components/otelcol/otelcol.processor.k8sattributes)
- [otelcol.processor.memory_limiter](../components/otelcol/otelcol.processor.memory_limiter)
- [otelcol.processor.metric_start_time](../components/otelcol/otelcol.processor.metric_start_time)
- [otelcol.processor.probabilistic_sampler](../components/otelcol/otelcol.processor.probabilistic_sampler)
- [otelcol.processor.resourcedetection](../components/otelcol/otelcol.processor.resourcedetection)
- [otelcol.processor.span](../components/otelcol/otelcol.processor.span)
- [otelcol.processor.tail_sampling](../components/otelcol/otelcol.processor.tail_sampling)
- [otelcol.processor.transform](../components/otelcol/otelcol.processor.transform)
- [otelcol.receiver.awscloudwatch](../components/otelcol/otelcol.receiver.awscloudwatch)
- [otelcol.receiver.awsecscontainermetrics](../components/otelcol/otelcol.receiver.awsecscontainermetrics)
- [otelcol.receiver.awss3](../components/otelcol/otelcol.receiver.awss3)
- [otelcol.receiver.cloudflare](../components/otelcol/otelcol.receiver.cloudflare)
- [otelcol.receiver.datadog](../components/otelcol/otelcol.receiver.datadog)
- [otelcol.receiver.faro](../components/otelcol/otelcol.receiver.faro)
- [otelcol.receiver.file_stats](../components/otelcol/otelcol.receiver.file_stats)
- [otelcol.receiver.filelog](../components/otelcol/otelcol.receiver.filelog)
- [otelcol.receiver.fluentforward](../components/otelcol/otelcol.receiver.fluentforward)
- [otelcol.receiver.googlecloudpubsub](../components/otelcol/otelcol.receiver.googlecloudpubsub)
- [otelcol.receiver.influxdb](../components/otelcol/otelcol.receiver.influxdb)
- [otelcol.receiver.jaeger](../components/otelcol/otelcol.receiver.jaeger)
- [otelcol.receiver.kafka](../components/otelcol/otelcol.receiver.kafka)
- [otelcol.receiver.loki](../components/otelcol/otelcol.receiver.loki)
- [otelcol.receiver.otlp](../components/otelcol/otelcol.receiver.otlp)
- [otelcol.receiver.prometheus](../components/otelcol/otelcol.receiver.prometheus)
- [otelcol.receiver.solace](../components/otelcol/otelcol.receiver.solace)
- [otelcol.receiver.splunkhec](../components/otelcol/otelcol.receiver.splunkhec)
- [otelcol.receiver.syslog](../components/otelcol/otelcol.receiver.syslog)
- [otelcol.receiver.tcplog](../components/otelcol/otelcol.receiver.tcplog)
- [otelcol.receiver.vcenter](../components/otelcol/otelcol.receiver.vcenter)
- [otelcol.receiver.zipkin](../components/otelcol/otelcol.receiver.zipkin)
{{< /collapse >}}

<!-- END GENERATED SECTION: CONSUMERS OF OpenTelemetry `otelcol.Consumer` -->

## Pyroscope `ProfilesReceiver`

The Pyroscope profiles are sent between components using `ProfilesReceiver`s.
`ProfilesReceiver`s are [capsules][] that are exported by components that can receive Pyroscope profiles.
Components that can consume Pyroscope profiles can be passed the `ProfilesReceiver` as an argument.
Use the following components to build your Pyroscope profiles pipeline:

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Pyroscope `ProfilesReceiver` Exporters

The following components, grouped by namespace, _export_ Pyroscope `ProfilesReceiver`.

<!-- START GENERATED SECTION: EXPORTERS OF Pyroscope `ProfilesReceiver` -->

{{< collapse title="pyroscope" >}}
- [pyroscope.enrich](../components/pyroscope/pyroscope.enrich)
- [pyroscope.relabel](../components/pyroscope/pyroscope.relabel)
- [pyroscope.write](../components/pyroscope/pyroscope.write)
{{< /collapse >}}

<!-- END GENERATED SECTION: EXPORTERS OF Pyroscope `ProfilesReceiver` -->

<!-- NOTE: this title is used as an anchor in links. Do not change. -->
### Pyroscope `ProfilesReceiver` Consumers

The following components, grouped by namespace, _consume_ Pyroscope `ProfilesReceiver`.

<!-- START GENERATED SECTION: CONSUMERS OF Pyroscope `ProfilesReceiver` -->

{{< collapse title="pyroscope" >}}
- [pyroscope.ebpf](../components/pyroscope/pyroscope.ebpf)
- [pyroscope.enrich](../components/pyroscope/pyroscope.enrich)
- [pyroscope.java](../components/pyroscope/pyroscope.java)
- [pyroscope.receive_http](../components/pyroscope/pyroscope.receive_http)
- [pyroscope.relabel](../components/pyroscope/pyroscope.relabel)
- [pyroscope.scrape](../components/pyroscope/pyroscope.scrape)
{{< /collapse >}}

<!-- END GENERATED SECTION: CONSUMERS OF Pyroscope `ProfilesReceiver` -->
