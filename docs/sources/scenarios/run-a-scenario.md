---
canonical: https://grafana.com/docs/alloy/latest/scenarios/run-a-scenario/
description: Clone the alloy-scenarios repository and run a Grafana Alloy scenario
menuTitle: Run a scenario
review_date: 2026-11-17
title: Run a scenario
weight: 50
---

# Run a scenario

Every scenario is in the [alloy-scenarios][] repository.
Each scenario directory has its own configuration, backends, and README.

## Before you begin

Ensure you have the following:

- [Docker][] and [Docker Compose][] for Docker-based scenarios
- Git
- kind, kubectl, and Helm for Kubernetes scenarios

## Start a Docker-based scenario

1. Clone the repository and change to its root directory:

   ```shell
   git clone https://github.com/grafana/alloy-scenarios.git
   cd alloy-scenarios
   ```

1. Start the scenario with the image versions pinned in `image-versions.env`:

   ```shell
   ./run-example.sh <SCENARIO_DIRECTORY>
   ```

   Replace _`<SCENARIO_DIRECTORY>`_ with the scenario path, for example `logs-file`.

1. Open Grafana at [http://localhost:3000][grafana-local] and the {{< param "PRODUCT_NAME" >}} UI at [http://localhost:12345][alloy-local].
   You don't need to log in to Grafana.
   Refer to the scenario README for additional endpoints.

## Stop a scenario

From the repository root, run:

```shell
cd <SCENARIO_DIRECTORY> && docker compose down
```

## Start a Kubernetes scenario

Kubernetes scenarios are in the `k8s/` directory.
Follow the README in the scenario directory.

[alloy-scenarios]: https://github.com/grafana/alloy-scenarios
[Docker]: https://docs.docker.com/get-docker/
[Docker Compose]: https://docs.docker.com/compose/install/
[grafana-local]: http://localhost:3000
[alloy-local]: http://localhost:12345
