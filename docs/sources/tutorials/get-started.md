---
canonical: https://grafana.com/docs/alloy/latest/tutorials/get-started/
description: Getting started with the tutorials
title: Get started
weight: 10
---

## Get started with {{% param "PRODUCT_NAME" %}}

This set of tutorials contains a collection of examples that build on each other to demonstrate how to configure and use [{{< param "PRODUCT_NAME" >}}][alloy].
To follow these tutorials, you need to have a basic understanding of what {{< param "PRODUCT_NAME" >}} is and telemetry collection in general.
You should also be familiar with with Prometheus and PromQL, Loki and LogQL, and basic Grafana navigation.
You don't need to know about the {{< param "PRODUCT_NAME" >}} [configuration syntax][configuration] concepts.

## Prerequisites

The tutorials require a Linux or Unix environment with Docker installed.
The examples run on a single host so that you can run them on your laptop or in a Virtual Machine.
You are encouraged to try the examples using a `config.alloy` file and experiment with the examples yourself.

To run the examples, you should have an {{< param "PRODUCT_NAME" >}} binary available. You can follow the instructions on how to [Install {{< param "PRODUCT_NAME" >}} as a Standalone Binary][install] to get a binary.

## Set up a local Grafana instance

You can use the following Docker Compose file to set up a local Grafana instance alongside Loki and Prometheus which are pre-configured as datasources. You can run and experiment with the examples on your local system.

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

The tutorials are designed to be followed in order and generally build on each other. Each example explains what it does and how it works.

The Recommended Reading sections in each tutorial provide a list of documentation topics. Read the recommended topics in the order given to help you understand the concepts used in the example.

[alloy]: https://grafana.com/docs/alloy/latest/
[configuration]: ../../concepts/configuration-syntax/
[install]: ../../get-started/install/binary/#install-alloy-as-a-standalone-binary
