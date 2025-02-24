# Changelog

> _Contributors should read our [contributors guide][] for instructions on how
> to update the changelog._

This document contains a historical list of changes between releases. Only
changes that impact end-user behavior are listed; changes to documentation or
internal API changes are not present.

0.12.0 (2025-02-24)
----------

### Enhancements

- Update to Grafana Alloy v1.7.0. (@thampiotr)

0.11.0 (2025-01-23)
----------

### Enhancements

- Update jimmidyson/configmap-reload to 0.14.0. (@petewall)
- Add the ability to deploy extra manifest files. (@dbluxo)

0.10.1 (2024-12-03)
----------

### Enhancements

- Update to Grafana Alloy v1.5.1. (@ptodev)

0.10.0 (2024-11-13)
----------

### Enhancements

- Add support for adding hostAliases to the Helm chart. (@duncan485)
- Update to Grafana Alloy v1.5.0. (@thampiotr)

0.9.2 (2024-10-18)
------------------

### Enhancements

- Update to Grafana Alloy v1.4.3. (@ptodev)

0.9.1 (2024-10-04)
------------------

### Enhancements

- Update to Grafana Alloy v1.4.2. (@ptodev)

0.9.0 (2024-10-02)
------------------

### Enhancements

- Add lifecyle hook to the Helm chart. (@etiennep)
- Add terminationGracePeriodSeconds setting to the Helm chart. (@etiennep)

0.8.1 (2024-09-26)
------------------

### Enhancements

- Update to Grafana Alloy v1.4.1. (@ptodev)

0.8.0 (2024-09-25)
------------------

### Enhancements

- Update to Grafana Alloy v1.4.0. (@ptodev)

0.7.0 (2024-08-26)
------------------

### Enhancements

- Add PodDisruptionBudget to the Helm chart. (@itspouya)

0.6.1 (2024-08-23)
----------

### Enhancements

- Add the ability to set --cluster.name in the Helm chart with alloy.clustering.name. (@petewall)
- Add the ability to set appProtocol in extraPorts to help OpenShift users to expose gRPC. (@clementduveau)

### Other changes

- Update helm chart to use v1.3.1.


0.6.0 (2024-08-05)
------------------

### Other changes

- Update helm chart to use v1.3.0.

- Set `publishNotReadyAddresses` to `true` in the service spec for clustering to fix a bug where peers could not join on startup. (@wildum)

0.5.1 (2023-07-11)
------------------

### Other changes

- Update helm chart to use v1.2.1.

0.5.0 (2024-07-08)
------------------

### Enhancements

- Only utilize spec.internalTrafficPolicy in the Service if deploying to Kubernetes 1.26 or later. (@petewall)

0.4.0 (2024-06-26)
------------------

### Enhancements

- Update to Grafana Alloy v1.2.0. (@ptodev)

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
