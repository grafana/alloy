# Docker Compose example

This directory contains a Docker Compose environment that can be used to test
Grafana Alloy.

> **NOTE**: This environment is not intended for production use, and is
> maintained on a best-effort basis.

By default, only Grafana and databases are exposed:

* Grafana, for visualizing telemetry (`localhost:3000`)
* Grafana Mimir, for storing metrics (`localhost:9009`)
* Grafana Loki, for storing logs (`localhost:3100`)
* Grafana Tempo, for storing traces (`localhost:3200`)
* Grafana Pyroscope, for storing profiles (`localhost:4040`)

Grafana is automatically provisioned with the appropriate datasources and
dashboards for monitoring Grafana Alloy.

To start the environment, run:

```bash
docker compose up -d
```

To stop the environment, run:

```bash
docker compose down
```

## Running Alloy

Alloy can either be run locally or within Docker Compose. The [example
configuration](./config/alloy/config.alloy) can be used to send self-monitoring
data from a local Alloy to the various databases running in Docker Compose.

To run Alloy within Docker Compose, pass `--profile=alloy` to `docker compose`
when starting and stopping the environment:

```bash
docker compose --profile=alloy up -d
```

```bash
docker compose --profile=alloy down
```

## Visualizing

To visualize Alloy data in Grafana, open <http://localhost:3000> in a web
browser and look at the dashboards in the `Alloy` folder.

> **NOTE**: It can take up to a minute for Alloy metrics and profiles to start
> appearing.
