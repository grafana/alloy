---
aliases:
- /docs/grafana-cloud/agent/flow/tutorials/flow-by-example/faq/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tutorials/flow-by-example/faq/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tutorials/flow-by-example/faq/
- /docs/grafana-cloud/send-data/agent/flow/tutorials/flow-by-example/faq/
canonical: https://grafana.com/docs/agent/latest/flow/tutorials/flow-by-example/faq/
description: Getting started with Flow-by-Example Tutorials
title: Get started
weight: 10
---

## Who is this for?

This set of tutorials contains a collection of examples that build on each other to demonstrate how to configure and use [{{< param "PRODUCT_NAME" >}}][flow]. It assumes you have a basic understanding of what {{< param "PRODUCT_ROOT_NAME" >}} is and telemetry collection in general. It also assumes a base level of familiarity with Prometheus and PromQL, Loki and LogQL, and basic Grafana navigation. It assumes no knowledge of {{< param "PRODUCT_NAME" >}} or River concepts.

[flow]: https://grafana.com/docs/agent/latest/flow

## What is Flow?

Flow is a new way to configure {{< param "PRODUCT_NAME" >}}. It is a declarative configuration language that allows you to define a pipeline of telemetry collection, processing, and output. It is built on top of the [River](https://github.com/grafana/river) configuration language, which is designed to be fast, simple, and debuggable.

## What do I need to get started?

You will need a Linux or Unix environment with Docker installed. The examples are designed to be run on a single host so that you can run them on your laptop or in a VM. You are encouraged to follow along with the examples using a `config.river` file and experiment with the examples yourself.

To run the examples, you should have a Grafana Agent binary available. You can follow the instructions on how to [Install Grafana Agent as a Standalone Binary](https://grafana.com/docs/agent/latest/flow/setup/install/binary/#install-grafana-agent-in-flow-mode-as-a-standalone-binary) to get a binary.

## How should I follow along?

You can use this docker-compose file to set up a local Grafana instance alongside Loki and Prometheus pre-configured as datasources. The examples are designed to be run locally, so you can follow along and experiment with them yourself.

```yaml
version: '3'
services:
  loki:
    image: grafana/loki:2.9.0
    ports:
      - "3100:3100"
    command: -config.file=/etc/loki/local-config.yaml
  prometheus:
    image: prom/prometheus:v2.47.0
    command:
      - --web.enable-remote-write-receiver
      - --config.file=/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
  grafana:
    environment:
      - GF_PATHS_PROVISIONING=/etc/grafana/provisioning
      - GF_AUTH_ANONYMOUS_ENABLED=true
      - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
    entrypoint:
      - sh
      - -euc
      - |
        mkdir -p /etc/grafana/provisioning/datasources
        cat <<EOF > /etc/grafana/provisioning/datasources/ds.yaml
        apiVersion: 1
        datasources:
        - name: Loki
          type: loki
          access: proxy
          orgId: 1
          url: http://loki:3100
          basicAuth: false
          isDefault: false
          version: 1
          editable: false
        - name: Prometheus
          type: prometheus
          orgId: 1
          url: http://prometheus:9090
          basicAuth: false
          isDefault: true
          version: 1
          editable: false
        EOF
        /run.sh
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
```

After running `docker-compose up`, open [http://localhost:3000](http://localhost:3000) in your browser to view the Grafana UI.

The tutorials are designed to be followed in order and generally build on each other. Each example explains what it does and how it works. They are designed to be run locally, so you can follow along and experiment with them yourself.

The Recommended Reading sections in each tutorial provide a list of documentation topics. To help you understand the concepts used in the example, read the recommended topics in the order given.
