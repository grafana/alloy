# Changelog

> _Contributors should read our [contributors guide][] for instructions on how
> to update the changelog._

This document contains a historical list of changes between releases. Only
changes that impact end-user behavior are listed; changes to documentation or
internal API changes are not present.

Unreleased
----------

0.3.2 (2024-05-30)
------------------

### Bugfixes

- Update to Grafana Alloy v1.1.1. (@rfratto)

0.3.1 (2024-05-22)
------------------

### Bugfixes

- Fix clustering on instances running within Istio mesh by allowing to change the name of the clustering port

0.3.0 (2024-05-14)
------------------

### Enhancements

- Update to Grafana Alloy v1.1.0. (@rfratto)


0.2.0 (2024-05-08)
------------------

### Other changes

- Support all [Kubernetes recommended labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/) (@nlamirault)

0.1.1 (2024-04-11)
------------------

### Other changes

- Add missing Alloy icon to Chart.yaml. (@rfratto)

0.1.0 (2024-04-09)
------------------

### Features

- Introduce a Grafana Alloy Helm chart. The Grafana Alloy Helm chart is
  backwards compatibile with the values.yaml from the `grafana-agent` Helm
  chart. Review the Helm chart README for a description on how to migrate.
  (@rfratto)
