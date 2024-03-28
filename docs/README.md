# Grafana Alloy Documentation

This directory contains documentation for Grafana Alloy. It is split into the
following parts:

* `sources/`: Source of user-facing documentation. This directory is hosted on
  [grafana.com/docs/agent](https://grafana.com/docs/agent/latest/), and we
  recommend viewing it there instead of the markdown on
  GitHub.
* `developer/`: Documentation for contributors and maintainers.
* `rfcs/`: RFCs for proposals relating to Grafana Agent.
* `generator/`: Code for generating some parts of the documentation.

## Preview the website

Run `make docs`. This launches a preview of the website with the current grafana docs at `http://localhost:3002/docs/alloy/latest/` which will refresh automatically when changes are made to content in the `sources` directory.
Make sure Docker is running.

## Update cloudwatch docs

First, inside the `docs/` folder run `make check-cloudwatch-integration` to verify that the cloudwatch docs needs updating.

If the check fails, then the doc supported services list should be updated. For that, run `make generate-cloudwatch-integration` to get the updated list, which should replace the old one in [the docs](./sources/static/configuration/integrations/cloudwatch-exporter-config.md).

## Update generated reference docs

Some sections of Grafana Alloy reference documentation are automatically generated. To update them, run `make generate-docs`.

### Community Projects

The following is a list of community-led projects for working with Grafana Alloy. These projects aren't maintained or supported by Grafana Labs.

<!--
#### Juju (Charmed Operator)

The [grafana-agent-k8s](https://github.com/canonical/grafana-agent-operator) charmed operator runs with [Juju](https://juju.is) the Grafana Agent on Kubernetes.
The Grafana Agent charmed operator is designed to work with the [Logs, Metrics and Alerts](https://juju.is/docs/lma2) observability stack.
-->