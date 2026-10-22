<p align="center">
  <img src="https://example.invalid/banner.png" alt="Banner" width="300"/>
</p>

# Grafana Alloy scenarios

This intro is repository-specific and the documentation pages replace it.

## Before you begin

Ensure you have the following:

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose.
- Git, to clone the repository.

## Run a scenario

Navigate to the scenario directory and open the README file.

From the repository root, start a scenario with `./run-example.sh <scenario-dir>`.

## Scenarios

Browse scenarios by telemetry type.

<!-- alloy-docs: slug=logs -->

### Logs

These scenarios focus on log collection and parsing.

| Scenario | Description |
| -------- | ----------- |
| [Logs from file](logs-file/) | Tail log files with Alloy. |
| [Structured log parsing](mail-house/) | Parse structured logs into labels. |

<!-- alloy-docs: slug=opentelemetry-engine -->

### OpenTelemetry Engine

These scenarios use OpenTelemetry YAML syntax.

Refer to the [OpenTelemetry examples README](otel-examples/) for details.

| Scenario | Description |
| -------- | ----------- |
| [Cost control](otel-examples/cost-control/) | Drop health checks and sample telemetry. |

## Contribute

This section stays on GitHub and is never imported.

## License

Apache 2.0.
