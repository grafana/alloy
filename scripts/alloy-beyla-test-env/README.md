# Alloy + Beyla Docker test environment

A small Docker-based environment to run Alloy with **beyla.ebpf**, **prometheus.exporter.unix**, and **prometheus.echo** for local testing. Metrics are printed to stdout (no database).

## Requirements

- Go (to run the app)
- Docker
- Optional: run as root or with access to `--privileged` (needed for beyla.ebpf eBPF)

## Usage

From the repo root:

```bash
go run ./scripts/alloy-beyla-test-env/ [CONFIG_DIR]
```

From this directory:

```bash
go run . [CONFIG_DIR]
```

- **CONFIG_DIR** (optional): host directory used as Alloy config (default: `./alloy-beyla-config`). Must contain `config.alloy`.
- On first run, a default `config.alloy` is created in that directory if missing.

The app mounts CONFIG_DIR into the container at `/host-alloy-config` (so the Alloy package can install to `/etc/alloy`). You can edit `config.alloy` on the host and restart the container to apply changes.

## Environment variables

- **ALLOY_VERSION** – Alloy version to install (default: `1.13.2`)
- **CONTAINER_NAME** – Docker container name (default: `alloy-beyla-test`)
- **IMAGE** – Base image (default: `ubuntu:22.04`)

## Config overview

- **beyla.ebpf**: discovers and instruments processes by open ports (default: 8080, 9090, 80, 443); application metrics only.
- **prometheus.exporter.unix**: node exporter metrics.
- **prometheus.echo**: receives scraped metrics and prints them to stdout in Prometheus text format.

Scrape intervals are 15s by default; you can change them in `config.alloy`.
