# Proposal: Zone-Aware Metrics Scraping

* Author(s): Timon Engelke
* Last updated: 2026-04-24
* Proposal issue: https://github.com/grafana/alloy/pull/6010

## Abstract

This proposal introduces a way for zone-aware metrics scraping by considering
the zone of a metrics target when choosing the Alloy instance that scrapes it.

## Problem

Currently, in a distributed Alloy setup with clustering enabled, scrape targets
are distributed to Alloy instances using a hash ring. The target distribution
is deterministic based on the name of the Alloy instance and the scrape target
labels, but does not take any properties of the scrape target or Alloy instance
into account.

In production setups, it is common to deploy software distributed across
multiple availability zones to ensure high availability in case of datacenter
failures. In these setups, Alloy scraping also crosses the zone boundaries. The
resulting cross-zone traffic is potentially costly (in case of some cloud
providers), can have slightly higher latency, and does not serve any redundancy
purpose -- when a zone becomes unavailable, its targets can no longer be
scraped no matter where the scraping Alloy instance is located; additionally,
targets in unaffected zones have to be redistributed to the remaining
instances.

For this reason, Alloy should support zone-aware metrics scraping. In this
setup, clustering should be set up in such a way that cross-zone metrics
scraping is avoided and Alloy instances only scrape targets that are in the
same availability zone. Targets that do not have an availability zone assigned,
such as external services, should fall back to the global (cross-zone) hash
ring.

This proposal only deals with scrape targets discovered by the
`prometheus.operator.podmonitors` and `prometheus.operator.servicemonitors`
components. Targets added directly via `prometheus.scrape` are excluded because
they do not carry the necessary metadata for zone discovery.

## Option 1: Local Zone Rings

In this option, each Alloy instance is aware of its own availability zone, e.g.
by reading the `topology.kubernetes.io/zone` label on its node. It also
retrieves the zone information of all its peers in a similar manner. With the
peers in the same zone, a dedicated hash ring (the local ring) is created. For
all scrape targets, the zone is also retrieved. This is possible by reading the
`__meta_kubernetes_pod_node_name` label and requesting the zone label of this
node. Now, all scrape targets that match an instance's zone are added to its
local ring. All scrape targets with a different zone are dropped and the scrape
targets without a zone are added to the global ring.
The option is enabled by setting `zone_aware = true` on the existing
`clustering` block and is disabled by default.

### Pros and cons

On the pro side, it is very easy for users to activate the feature because only
a single option has to be changed in an existing setup.

A disadvantage of the solution is that if there is a zone without Alloy
instances, targets in that zone are not scraped as they don't fall through to
the global ring. The user therefore has to make sure that the Alloy instances
are distributed across all zones, e.g. via topologySpreadConstraints and/or
PodDisruptionBudgets.

The option also increases the number of calls to the Kubernetes API because a
map from nodes to zones has to be created and kept up-to-date. Node changes
are, however, probably rare enough for this to be reasonable, especially
because every pod or endpoint change already has to be monitored.

### Compatibility

The option is completely backwards-compatible because the option is disabled by
default.

### Implementation

A reference implementation is ready for discussion in
https://github.com/grafana/alloy/pull/6010.

## Option 2: Isolated Deployments

In this option, no Alloy code changes are made. Instead, zone-awareness
is set up using isolated deployments. This means that for each zone, a
separate Alloy deployment is set up. Each deployment has a dedicated
configuration that filters scrape targets based on their availability zone. A
separate deployment that filters for missing zone information is responsible
for external services. Zone information for scrape targets can be fetched from
Kubernetes by setting `attach_metadata` to `node` on the PodMonitors and
reading `__meta_kubernetes_node_label_topology_kubernetes_io_zone` or by
reading `__meta_kubernetes_endpointslice_endpoint_zone` for ServiceMonitors. An
Alloy instance can learn its zone from an environment variable.

### Pros and cons

This setup requires no code changes and is therefore guaranteed to not break
exsting setups. Also, the isolated deployments allow for independent scaling in
the different zones.

However, the configuration setup can be cumbersome for users that do not want
to dive into the details of Alloy scraping behavior. For example, the
`attach_metadata` option has to be set on all PodMonitors individually. Good
documentation and examples are necessary, but it is still be possible that the
feature remains niche.

Like in Option 1, users have to make sure that Alloy is deployed in all zones.
Otherwise, targets in that zone would not be scraped.

### Compatibility

This option is completely backwards-compatible because no code changes are done.

### Implementation

Good documentation and examples are required, as well as an option for the
separate deployments in the Helm Chart.

Another option with minimal implementation changes is also be possible, for
example by setting `attach_metadata` programmatically on the PodMonitors.
