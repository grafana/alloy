# Grafana Alloy Documentation

This directory contains documentation for Grafana Alloy. It is split into the following parts:

* `sources/`: Source of user-facing documentation.
  This directory is hosted on [grafana.com/docs/alloy](https://grafana.com/docs/alloy/latest/), and we recommend viewing it there instead of the markdown on GitHub.
* `developer/`: Documentation for contributors and maintainers.

## Preview the website

Run `make docs`.
This launches a preview of the website with the current grafana docs at `http://localhost:3002/docs/alloy/latest/` which automatically refreshes when changes are made to content in the `sources` directory.
Make sure Docker is running.

## Update CloudWatch docs

From the repository root, run `task docs:cloudwatch-sync` to update the supported services list in the [CloudWatch exporter documentation](./sources/reference/components/prometheus/prometheus.exporter.cloudwatch.md).

## Update generated reference docs

Some sections of Grafana Alloy reference documentation are automatically generated. To update them, run `make generate-docs`.
