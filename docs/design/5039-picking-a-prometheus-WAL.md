# Proposal: Picking a Prometheus WAL

* Author(s): Kyle Eckhart (@kgeckhart)
* Last updated: 2025-12-09
* Original issue: https://github.com/grafana/alloy/issues/5039

## Abstract

As of today there are two write-ahead-log (WAL) implementations for prometheus metrics in Alloy

[prometheus.remote_write](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.remote_write/) (referred to as remote_write)
- Largely based on upstream prometheus concepts in the “[head_WAL](https://github.com/prometheus/prometheus/blob/main/tsdb/head_wal.go)”
- Perceived as inheriting concepts that are relevant for a database, IE caring about “active series”, which is an implementation tradeoff rather than a database concept that was inherited
- One of the most used components in Alloy

[prometheus.write.queue](https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.write.queue/) (referred to as write.queue)
- Built to solve issues related to remote write such as,
    - Reduce memory overhead as “active series” increase
    - Add ability to replay data on startup
- Custom implementation based on https://github.com/grafana/walqueue/ with no links to upstream prometheus
- Currently, an experimental component with low usage

## Problem

As Alloy continues to get closer and closer to the Open Telemetry (OTel) Collector we need to ensure we have a clear and concise story around our various unique capabilities. The OTel Collector also has two WAL implementations,

* [WAL for all signals](https://opentelemetry.io/docs/collector/resiliency/#persistent-storage-write-ahead-log---wal): sending_queue with the file_storage extension
* Optional WAL for the [prometheus remote write exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/prometheusremotewriteexporter#getting-started)

The intention was that we try to upstream WALQueue and it would become the basis for prometheus and OTel. 
As of today, upstreaming conversations have not started. 
Realistically, convincing OTel to accept a third WAL implementation is unlikely, and we will face similar challenges for the Prometheus ecosystem. 
There are other non-trivial investments required for write.queue including,
* Solving scaling issues encountered with implementing Prometheus Remote Write v2
* Investing enough to feel comfortable transitioning the component from experimental -> public preview -> GA
* Deciding on a migration strategy / determining how best to wrap remote_write to use write.queue under the hood

**Goals:**

Primary: Reduce the amount of WAL implementations the Alloy team is responsible for
Primary: Avoid a migration for customers currently using the remote_write component
Secondary: Gauge how the OTel WAL implementations compare to Alloy’s
*  Having this data will allow us to write up pros and cons for choosing a “prometheus native” pipeline vs an “OTel native” pipeline with metrics
*  It also will give us a lot of data we can use to benefit both ecosystems via improvements to the OTel collector WAL implementations and/or the prometheus remote write exporter WAL

## Proposal 0: Do Nothing

We continue to provide support for multiple WAL implementations increasing toil.

This is not a viable options and accomplishes none of the goals.

## Proposal 1: Improve remote_write to be the WAL we need

There are some big sticking points with remote_write
1. Data can be removed from the WAL before it is sent
1. Resource utilization spikes incredibly high on startup and spikes during normal usage
1. Memory scales in a non-linear fashion as the number of series grows
1. Cannot replay data on restart

All of these are resolvable issues with some upstream work or on unique alloy capabilities such as the LabelStore. Many were done in a PoC fashion during a Grafana Labs hackathon.

As a part of this we would fully move to wrapping the upstream prometheus components instead of maintaining our current soft-fork.

**Pros**
* It is already upstream in prometheus and improvements we make can be upstreamed
* Customers already use it
* Moving to 100% upstream will require lower maintenance burden over time

**Cons**
* We can improve memory overhead but we might not get as memory efficient as write.queue
  * Total cost of ownership should be a wash in the end as write.queue is more CPU heavy.
* Since we are dependent on upstream decisions some of the more complex efforts, like replay, could take more time

## Proposal 2: Invest in write.queue as our WAL

As an Alloy WAL implementation, write.queue solves the main sticking points for remote_write with a scaling profile that’s more predictable as the number of series increases and can accomplish replay*. 

* The caveat with the implementation and replay is that it’s not  “transactional”. As soon as the data is read to be distributed for delivery it is removed from disk leaving a hole where if data fails to be delivered it will be lost. This is a similar challenge that remote_write faces, it is viewed as a requirement for a replay option in remote_write. Dropping this requirement dramatically reduces the complexity of a replay solution. 

There are some concerns though regarding the level of investment to feel comfortable transitioning the component from experimental -> public preview and further on the work it would take to migrate our existing remote_write customers.

**Pros**

* More consistent resource utilization + replay* OOTB

**Cons**

* Designed with Prometheus remote write v1 in mind causing friction when implementing Prometheus remote write v2
* Unless we can upstream it, it’s 100% on us to maintain
* We need to consider how to migrate existing customers off of remote_write

## Compatibility

Changes proposed in Proposal 1 will be backwards compatible. Proposal 2 would either require breaking changes or deciding how best to wrap `prometheus.write.queue` to replace the existing `prometheus.remote_write` component.

## Implementation

1. Data can be removed from the WAL before it is sent: https://github.com/prometheus/prometheus/issues/17616 is the upstream proposal
1. Resource utilization spikes incredibly high on startup and spikes during normal usage: https://github.com/prometheus/prometheus/issues/17617 is the upstream proposal
1. Memory scales in a non-linear fashion as the number of series grows
  * There's an upstream proposal, https://github.com/prometheus/prometheus/issues/17619, to experiment to eliminate some dual caching of data between the WAL + queue_manager for metadata. If that is successful, we would do the same experiment with series reducing a lot of complexity/resources required for queue_manager to keep up to date with what series are active in the WAL
  * The second part of this is eliminating the overhead of LabelStore. A proposal which simplifies when/how LabelStore is used for prometheus will be created shortly. 
1. Cannot replay data on restart: Conversations have been started to get agreed upon requirements which will guide the first implementation but nothing to link to formally yet.

At the same time we would formally deprecate the `prometheus.write.queue` component in the next release.

## Consensus

We have decided to move forward with Proposal 1 and invest in the upstream Prometheus implementation to allow `prometheus.remote_write` to be the WAL we need.
We will move forward with deprecating `prometheus.write.queue` in the next release to be removed entirely in the future.
