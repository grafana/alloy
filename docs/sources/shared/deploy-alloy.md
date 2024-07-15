---
canonical: https://grafana.com/docs/alloy/latest/shared/deploy-alloy/
description: Shared content, deployment topologies for Grafana Alloy
headless: true
title: Deploy Grafana Alloy
---

# Deploy {{% param "FULL_PRODUCT_NAME" %}}

{{< param "PRODUCT_NAME" >}} is a flexible, vendor-neutral telemetry collector.
This flexibility means that {{< param "PRODUCT_NAME" >}} doesn’t enforce a specific deployment topology but can work in multiple scenarios.

This page lists common topologies used for {{% param "PRODUCT_NAME" %}} deployments, when to consider using each topology, issues you may run into, and scaling considerations.

## As a centralized collection service

Deploying {{< param "PRODUCT_NAME" >}} as a centralized service is recommended for collecting application telemetry.
This topology allows you to use a smaller number of collectors to coordinate service discovery, collection, and remote writing.

{{< figure src="/media/docs/alloy/collection-diagram-alloy.png" alt="Centralized collection with Alloy">}}

Using this topology requires deploying {{< param "PRODUCT_NAME" >}} on separate infrastructure, and making sure that they can discover and reach these applications over the network.
The main predictor for the size of an {{< param "PRODUCT_NAME" >}} deployment is the number of active metrics series it's scraping. A rule of thumb is approximately 10 KB of memory for each series.
We recommend you start looking towards horizontal scaling around the 1 million active series mark.

### Using Kubernetes StatefulSets

Deploying {{< param "PRODUCT_NAME" >}} as a StatefulSet is the recommended option for metrics collection.
The persistent Pod identifiers make it possible to consistently match volumes with pods so that you can use them for the WAL directory.

You can also use a Kubernetes Deployment in cases where persistent storage isn't required, such as a traces-only pipeline.

### Pros

* Straightforward scaling using [clustering][]
* Minimizes the “noisy neighbor” effect
* Easy to meta-monitor

### Cons

* Requires running on separate infrastructure

### Use for

* Scalable telemetry collection

### Don’t use for

* Host-level metrics and logs

## As a host daemon

Deploying one {{< param "PRODUCT_NAME" >}} instance per machine is required for collecting machine-level metrics and logs, such as node_exporter hardware and network metrics or journald system logs.

{{< figure src="/media/docs/alloy/host-diagram-alloy.png" alt="Alloy as a host daemon">}}

Each {{< param "PRODUCT_NAME" >}} instance requires you to open an outgoing connection for each remote endpoint it’s shipping data to.
This can lead to NAT port exhaustion on the egress infrastructure.
Each egress IP can support up to (65535 - 1024 = 64511) outgoing connections on different ports.
So, if all {{< param "PRODUCT_NAME" >}}s are shipping metrics and log data, an egress IP can support up to 32,255 collectors.

### Using Kubernetes DaemonSets

The simplest use case of the host daemon topology is a Kubernetes DaemonSet, and it's required for node-level observability (for example cAdvisor metrics) and collecting Pod logs.

### Pros

* Doesn’t require running on separate infrastructure
* Typically leads to smaller-sized collectors
* Lower network latency to instrumented applications

### Cons

* Requires planning a process for provisioning {{< param "PRODUCT_NAME" >}} on new machines, as well as keeping configuration up to date to avoid configuration drift
* Not possible to scale independently when using Kubernetes DaemonSets
* Scaling the topology can strain external APIs (like service discovery) and network infrastructure (like firewalls, proxy servers, and egress points)

### Use for

* Collecting machine-level metrics and logs (for example, node_exporter hardware metrics, Kubernetes Pod logs)

### Don’t use for

* Scenarios where {{< param "PRODUCT_NAME" >}} grows so large it can become a noisy neighbor
* Collecting an unpredictable amount of telemetry

## As a container sidecar

Deploying {{< param "PRODUCT_NAME" >}} as a container sidecar is only recommended for short-lived applications or specialized {{< param "PRODUCT_NAME" >}} deployments.

{{< figure src="/media/docs/alloy/sidecar-diagram-alloy.png" alt="Alloy as a container sidecar">}}

### Using Kubernetes Pod sidecars

In a Kubernetes environment, the sidecar model consists of deploying {{< param "PRODUCT_NAME" >}} as an extra container on the Pod.
The Pod’s controller, network configuration, enabled capabilities, and available resources are shared between the actual application and the sidecar {{< param "PRODUCT_NAME" >}}.

### Pros

* Doesn’t require running on separate infrastructure
* Straightforward networking with partner applications

### Cons

* Doesn’t scale separately
* Makes resource consumption harder to monitor and predict
* Each {{< param "PRODUCT_NAME" >}} instance doesn't have a life cycle of its own, making it harder to do things like recovering from network outages

### Use for

* Serverless services
* Job/batch applications that work with a push model
* Air-gapped applications that can’t be otherwise reached over the network

### Don’t use for

* Long-lived applications
* Scenarios where the {{< param "PRODUCT_NAME" >}} deployment size grows so large it can become a noisy neighbor

<!-- ToDo: Check URL path -->
[clustering]: ../../configure/clustering/
