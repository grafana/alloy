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

First, inside the `docs/` folder run `make check-cloudwatch-integration` to verify that the CloudWatch docs needs updating.

If the check fails, then the doc supported services list should be updated.
For that, run `make generate-cloudwatch-integration` to get the updated list, which should replace the old one in [the docs](./sources/static/configuration/integrations/cloudwatch-exporter-config.md).

## Update generated reference docs

Some sections of Grafana Alloy reference documentation are automatically generated. To update them, run `make generate-docs`.
