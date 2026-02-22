---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/healthcheck/
description: Learn about the healthcheck command
labels:
  stage: general-availability
  products:
    - oss
title: healthcheck
weight: 250
---

# `healthcheck`

The `healthcheck` command checks the health of a running {{< param "PRODUCT_NAME" >}} instance.

## Usage

```shell
alloy healthcheck [<FLAG> ...]
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the input and output of the command.

The `healthcheck` command queries a running {{< param "PRODUCT_NAME" >}} instance's health endpoint by making an HTTP `GET` request.
If the endpoint returns an `HTTP 200` status, the command exits with code `0`.
Otherwise, the command exits with a non-zero exit code.

By default, the command queries the `/-/ready` endpoint at `127.0.0.1:12345`.
Use `healthcheck` in Docker `HEALTHCHECK` instructions or other orchestration tools to monitor the health of {{< param "PRODUCT_NAME" >}} without external utilities like `curl` or `wget`.

The following flags are supported:

* `--url`: Full URL to check. Overrides `--addr` and `--path`.
* `--addr`: Address of the running {{< param "PRODUCT_NAME" >}} instance (default `"127.0.0.1:12345"`).
* `--path`: Path to the health endpoint (default `"/-/ready"`).
* `--timeout`: Timeout for the HTTP request (default `"5s"`).

## Docker HEALTHCHECK

To use `healthcheck` in a Docker container, add a `HEALTHCHECK` instruction to your Dockerfile:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["alloy", "healthcheck"]
```

If your {{< param "PRODUCT_NAME" >}} instance listens on a custom address, pass the `--addr` flag:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["alloy", "healthcheck", "--addr=0.0.0.0:9090"]
```

## Health endpoints

{{< param "PRODUCT_NAME" >}} exposes two health endpoints:

- **`/-/ready`**: Returns `HTTP 200` if {{< param "PRODUCT_NAME" >}} has loaded its initial configuration. Returns `HTTP 503` otherwise. This is the default endpoint used by `healthcheck`.
- **`/-/healthy`**: Returns `HTTP 200` if all components are healthy. Returns `HTTP 500` with the names of unhealthy components otherwise.

The `/-/ready` endpoint is recommended for container health checks because it indicates whether {{< param "PRODUCT_NAME" >}} is functional.
The `/-/healthy` endpoint checks individual component health, which can cause unnecessary container restarts during transient component issues.

For more information, refer to the [HTTP endpoints](../../http/) reference.
