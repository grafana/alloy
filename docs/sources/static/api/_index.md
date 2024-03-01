---
aliases:
- ../api/
- /docs/grafana-cloud/monitor-infrastructure/agent/static/api/
- /docs/grafana-cloud/send-data/agent/static/api/
canonical: https://grafana.com/docs/agent/latest/static/api/
description: Learn about the Grafana Agent static mode API
menuTitle: Static mode API
title: Static mode APIs (Stable)
weight: 400
---

# Static mode APIs (Stable)

The API for static mode is divided into several parts:

- [Config Management API](#config-management-api-beta)
- [Agent API](#agent-api)
- [Integrations API](#integrations-api-experimental)
- [Ready/Healthy API](#ready--health-api)

API endpoints are stable unless otherwise noted.

## Config management API (Beta)

Grafana Agent exposes a configuration management REST API for managing instance configurations when it's running in [scraping service mode][scrape].

{{< admonition type="note" >}}
The scraping service mode is a requirement for the configuration management
API, however this isn't a prerequisite for the Agent API or Ready/Healthy API.
{{< /admonition >}}

The following endpoints are exposed:

- List configs: [`GET /agent/api/v1/configs`](#list-configs)
- Get config: [`GET /agent/api/v1/configs/{name}`](#get-config)
- Update config: [`PUT /agent/api/v1/config/{name}`](#update-config)
- Delete config: [`DELETE /agent/api/v1/config/{name}`](#delete-config)

{{< admonition type="note" >}}
If you are running Grafana Agent in a Docker container and you want to expose the API outside the Docker container, you must change the default HTTP listen address from `127.0.0.1:12345` to a valid network interface address.
You can change the HTTP listen address with the command-line flag: `-server.http.address=0.0.0.0:12345`.
For more information, refer to the [Server](https://grafana.com/docs/agent/latest/static/configuration/flags/#server) command-line flag documentation.

You must also publish the port in Docker. Refer to [Published ports](https://docs.docker.com/network/#published-ports) in the Docker documentation for more information.
{{< /admonition >}}

### API response

All Config Management API endpoints will return responses in the following
form, unless an internal service error prevents the server from responding
properly:

```
{
  "status": "success" | "error",
  "data": {}
}
```

Status will be either `success` or `error`. All 2xx responses will be
accompanied by a `success` value for the status field. 4xx and 5xx
responses will provide a value of `error`. All requests may potentially
return 500 on an internal error. Other non-500 responses will be documented
per API.

The data field may or may not be present, depending on the endpoint. It
provides extra information for the query. The documentation for each endpoint
will describe the full response provided.

### List configs

```
GET /agent/api/v1/configs
```

List configs returns a list of the named configurations currently known by the
underlying KV store.

Status code: 200 on success.
Response:

```
{
  "status": "success",
  "data": {
    "configs": [
      // list of config names:
      "a",
      "b",
      "c",
      // ...
    ]
  }
}
```

### Get config

```
GET /agent/api/v1/configs/{name}
```

Get config returns a single configuration by name. The configuration must
exist or an error will be returned. URL-encoded names will be retrieved in decoded
form. e.g., `hello%2Fworld` will represent the config named `hello/world`.

Status code: 200 on success, 400 on invalid config name.
Response on success:

```
{
  "status": "success",
  "data": {
    "value": "/* YAML configuration */"
  }
}
```

### Update config

```
PUT /agent/api/v1/config/{name}
POST /agent/api/v1/config/{name}
```

Update config updates or adds a new configuration by name. If a configuration
with the same name already exists, then it will be completely overwritten.

URL-encoded names are stored in decoded form. e.g., `hello%2Fworld` will
represent the config named `hello/world`.

The request body passed to this endpoint must match the format of [metrics_instance_config][metrics]
defined in the Configuration Reference. The name field of the configuration is
ignored and the name in the URL takes precedence. The request body must be
formatted as YAML.

{{< admonition type="warning" >}}
By default, all instance configuration files that read
credentials from a file on disk will be rejected. This prevents malicious users
from reading the contents of arbitrary files as passwords and sending their
contents to fake remote_write endpoints. To change the behavior, set
`dangerous_allow_reading_files` to true in the `scraping_service` block.
{{< /admonition >}}

Status code: 201 with a new config, 200 on updated config.
Response on success:

```
{
  "status": "success"
}
```

### Delete config

```
DELETE /agent/api/v1/config/{name}
```

Delete config attempts to delete a configuration by name. The named
configuration must exist; deleting a nonexistent config will result in an
error.

URL-encoded names will be interpreted in decoded form. e.g., `hello%2Fworld`
will represent the config named `hello/world`.

Status code: 200 on success, 400 with invalid config name.
Response on success:

```
{
  "status": "success"
}
```

## Agent API

### List current running instances of metrics subsystem

```
GET /agent/api/v1/metrics/instances
```

{{< admonition type="note" >}}
The deprecated alias is `/agent/api/v1/instances`
{{< /admonition >}}

Status code: 200 on success.
Response on success:

```
{
  "status": "success",
  "data": [
    <strings of instance names that are currently running>
  ]
}
```

### List current scrape targets of metrics subsystem

```
GET /agent/api/v1/metrics/targets
```

{{< admonition type="note" >}}
The deprecated alias is `/agent/api/v1/targets`
{{< /admonition >}}

This endpoint collects all metrics subsystem targets known to the Agent across all
running instances. Only targets being scraped from the local Agent will be returned. If
running in scraping service mode, this endpoint must be invoked in all Agents
separately to get the combined set of targets across the whole Agent cluster.

The `labels` fields shows the labels that will be added to metrics from the
target, while the `discovered_labels` field shows all labels found during
service discovery.

Status code: 200 on success.
Response on success:

```
{
  "status": "success",
  "data": [
    {
      "instance": <string, instance config name>,
      "target_group": <string, scrape config group name>,
      "endpoint": <string, URL being scraped>
      "state": <string, one of up, down, unknown>,
      "discovered_labels": {
        "__address__": "<address>",
        ...
      },
      "labels": {
        "label_a": "value_a",
        ...
      },
      "last_scrape": <string, RFC 3339 timestamp of last scrape>,
      "scrape_duration_ms": <number, last scrape duration in milliseconds>,
      "scrape_error": <string, last error. empty if scrape succeeded>
    },
    ...
  ]
}
```

### Accept remote_write requests

```
POST /agent/api/v1/metrics/instance/{instance}/write
```

This endpoint accepts Prometheus-compatible remote_write POST requests, and
appends their contents into an instance's WAL.

Replace `{instance}` with the name of the metrics instance from your config
file. For example, this block defines the "dev" and "prod" instances:

```yaml
metrics:
  configs:
  - name: dev     # /agent/api/v1/metrics/instance/dev/write
    ...
  - name: prod    # /agent/api/v1/metrics/instance/prod/write
    ...
```

Status code: 204 on success, 400 for bad requests related to the provided
instance or POST payload format and content, 500 for cases where appending
to the WAL failed.

### List current running instances of logs subsystem

```
GET /agent/api/v1/logs/instances
```

Status code: 200 on success.
Response on success:

```
{
  "status": "success",
  "data": [
    <strings of instance names that are currently running>
  ]
}
```

### List current scrape targets of logs subsystem

```
GET /agent/api/v1/logs/targets
```

This endpoint collects all logs subsystem targets known to the Agent across
all running instances. Only targets being scraped from Promtail will be returned.

The `labels` fields shows the labels that will be added to metrics from the
target, while the `discovered_labels` field shows all labels found during
service discovery.

Status code: 200 on success.
Response on success:

```
{
  "status": "success",
  "data": [
    {
      "instance": "default",
      "target_group": "varlogs",
      "type": "File",
      "labels": {
        "job": "varlogs"
      },
      "discovered_labels": {
        "__address__": "localhost",
        "__path__": "/var/log/*log",
        "job": "varlogs"
      },
      "ready": true,
      "details": {
        "/var/log/alternatives.log": 13386,
        "/var/log/apport.log": 0,
        "/var/log/auth.log": 37009,
        "/var/log/bootstrap.log": 107347,
        "/var/log/dpkg.log": 374420,
        "/var/log/faillog": 0,
        "/var/log/fontconfig.log": 11629,
        "/var/log/gpu-manager.log": 1541,
        "/var/log/kern.log": 782582,
        "/var/log/lastlog": 0,
        "/var/log/syslog": 788450
      }
    }
  ]
}
```

### Reload configuration file (beta)

This endpoint is currently in beta and may have issues. Please open any issues
you encounter.

```
GET /-/reload
POST /-/reload
```

This endpoint will re-read the configuration file from disk and refresh the
entire state of the Agent to reflect the new file on disk:

- HTTP Server
- Prometheus metrics subsystem
- Loki logs subsystem
- Tempo traces subsystem
- Integrations

Valid configurations will be applied to each of the subsystems listed above, and
`/-/reload` will return with a status code of 200 once all subsystems have been
updated. Malformed configuration files (invalid YAML, failed validation checks)
will be immediately rejected with a status code of 400.

Well-formed configuration files can still be invalid for various reasons, such
as not having permissions to read the WAL directory. Issues such as these will
cause per-subsystem problems while reloading the configuration, and will leave
that subsystem in an undefined state. Specific errors encountered during reload
will be logged, and should be fixed before calling `/-/reload` again.

Status code: 200 on success, 400 otherwise.

### Show configuration file

```
GET /-/config
```

This endpoint prints out the currently loaded configuration the Agent is using.
The returned YAML has defaults applied, and only shows changes to the state that
validated successfully, so the results will not identically match the
configuration file on disk.

Status code: 200 on success.

### Generate support bundle
```
GET /-/support?duration=N
```

This endpoint returns a 'support bundle', a zip file that contains information
about a running agent, and can be used as a baseline of information when trying
to debug an issue.

The duration parameter is optional, must be less than or equal to the
configured HTTP server write timeout, and if not provided, defaults to it.
The endpoint is only exposed to the agent's HTTP server listen address, which
defaults to `localhost:12345`.

The support bundle contains all information in plain text, so that it can be
inspected before sharing, to verify that no sensitive information has leaked.

In addition, you can inspect the [supportbundle package](https://github.com/grafana/agent/tree/main/internal/static/supportbundle)
to verify the code that is being used to generate these bundles.

A support bundle contains the following data:
* `agent-config.yaml` contains the current agent configuration (when the `-config.enable-read-api` flag is passed).
* `agent-logs.txt` contains the agent logs during the bundle generation.
* `agent-metadata.yaml` contains the agent's build version, operating system, architecture, uptime, plus a string payload defining which extra agent features have been enabled via command-line flags.
* `agent-metrics-instances.json` and `agent-metrics-targets.json` contain the active metric subsystem instances and the discovered scrape targets for each one.
* `agent-logs-instances.json` and `agent-logs-targets.json` contains the active logs subsystem instances and the discovered log targets for each one.
* `agent-metrics.txt` contains a snapshot of the agent's internal metrics.
* The `pprof/` directory contains Go runtime profiling data (CPU, heap, goroutine, mutex, block profiles) as exported by the pprof package.

## Integrations API (Experimental)

> **WARNING**: This API is currently only available when the experimental
> [integrations revamp][integrations]
> is enabled. Both the revamp and this API are subject to change while they
> are still experimental.

### Integrations SD API

```
GET /agent/api/v1/metrics/integrations/sd
```

This endpoint returns all running metrics-based integrations. It conforms to
the Prometheus [http_sd_config API](https://prometheus.io/docs/prometheus/2.45/configuration/configuration/#http_sd_config).
Targets include integrations regardless of autoscrape being enabled; this
allows for manually configuring scrape jobs to collect metrics from an
integration running on an external agent.

The following labels will be present on all returned targets:

- `instance`: The unique instance ID of the running integration.
- `job`: `integrations/<__meta_agent_integration_name>`
- `agent_hostname`: `hostname:port` of the agent running the integration.
- `__meta_agent_integration_name`: The name of the integration.
- `__meta_agent_integration_instance`: The unique instance ID for the running integration.
- `__meta_agent_integration_autoscrape`: `1` if autoscrape is enabled for this integration, `0` otherwise.

To reduce the load on the agent's HTTP server, the following query parameters
may also be provided to the URL:

- `integrations`: Comma-delimited list of integrations to return. i.e., `agent,node_exporter`.
- `instance`: Return all integrations matching a specific value for instance.

Status code: 200 if successful.
Response on success:

```
[
  {
    "targets": [ "<host>", ... ],
    "labels": {
      "<labelname>": "<labelvalue>", ...
    }
  },
  ...
]
```

### Integrations autoscrape targets

```
GET /agent/api/v1/metrics/integrations/targets
```

This endpoint returns all integrations for which autoscrape is enabled. The
response is identical to [`/agent/api/v1/metrics/targets`](#list-current-scrape-targets-of-logs-subsystem).

Status code: 200 on success.
Response on success:

```
{
  "status": "success",
  "data": [
    {
      "instance": <string, metrics instance where autoscraped metrics are sent>,
      "target_group": <string, scrape config group name>,
      "endpoint": <string, URL being scraped>
      "state": <string, one of up, down, unknown>,
      "discovered_labels": {
        "__address__": "<address>",
        ...
      },
      "labels": {
        "label_a": "value_a",
        ...
      },
      "last_scrape": <string, RFC 3339 timestamp of last scrape>,
      "scrape_duration_ms": <number, last scrape duration in milliseconds>,
      "scrape_error": <string, last error. empty if scrape succeeded>
    },
    ...
  ]
}
```

## Ready / health API

### Readiness check

```
GET /-/ready
```

Status code: 200 if ready.

Response:
```
Agent is Ready.
```

### Healthiness check

```
GET /-/healthy
```

Status code: 200 if healthy.

Response:
```
Agent is Healthy.
```

{{% docs/reference %}}
[scrape]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/scraping-service"
[scrape]: "/docs/grafana-cloud/ -> ../configuration/scraping-service
[metrics]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/metrics-config"
[metrics]: "/docs/grafana-cloud/ -> ../configuration/metrics-config"
[integrations]: "/docs/agent/ -> /docs/agent/<AGENT_VERSION>/static/configuration/integrations/integrations-next"
[integrations]: "/docs/grafana-cloud/ -> ../configuration/integrations/integrations-next"
{{% /docs/reference %}}
