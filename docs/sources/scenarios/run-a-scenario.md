---
canonical: https://grafana.com/docs/alloy/latest/scenarios/run-a-scenario/
description: Clone the alloy-scenarios repository and run a Grafana Alloy scenario
review_date: 2026-10-15
menuTitle: Run a scenario
title: Run a scenario
weight: 50
---

# Run a scenario

Every scenario is in the [alloy-scenarios][] repository.
Each scenario directory has its own configuration, backends, and README.

## Before you begin

Ensure you have the following:

- [Docker][] and [Docker Compose][].
- Git, to clone the repository.

## Start a scenario

Clone the repository:

```shell
git clone https://github.com/grafana/alloy-scenarios.git
```

Navigate to the scenario directory and open the README file.
The scenario README documents everything you need to know to deploy and explore the scenario.

From the repository root, you can start Docker-based scenarios with pinned image versions: `./run-example.sh <scenario-dir>`.
Image versions are in `image-versions.env`.

Kubernetes scenarios use Helm charts.

## Explore the services

Most Docker-based scenarios expose Grafana at http://localhost:3000 and the Alloy UI at http://localhost:12345.
You don't need to log in to Grafana in these scenarios.
Refer to the scenario README for additional endpoints.

[alloy-scenarios]: https://github.com/grafana/alloy-scenarios
[Docker]: https://docs.docker.com/get-docker/
[Docker Compose]: https://docs.docker.com/compose/install/
