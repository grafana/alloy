---
aliases:
- /docs/grafana-cloud/agent/flow/reference/config-blocks/import.http/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/reference/config-blocks/import.http/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/reference/config-blocks/import.http/
- /docs/grafana-cloud/send-data/agent/flow/reference/config-blocks/import.http/
canonical: https://grafana.com/docs/agent/latest/flow/reference/config-blocks/import.http/
description: Learn about the import.http configuration block
labels:
  stage: beta
title: import.http
---

# import.http

{{< docs/shared lookup="flow/stability/beta.md" source="agent" version="<AGENT_VERSION>" >}}

`import.http` retrieves a module from an HTTP server.

## Usage

```river
import.http "LABEL" {
  url = URL
}
```

## Arguments

The following arguments are supported:

Name             | Type          | Description                             | Default | Required
-----------------|---------------|-----------------------------------------|---------|---------
`url`            | `string`      | URL to poll.                            |         | yes
`method`         | `string`      | Define the HTTP method for the request. | `"GET"` | no
`headers`        | `map(string)` | Custom headers for the request.         | `{}`    | no
`poll_frequency` | `duration`    | Frequency to poll the URL.              | `"1m"`  | no
`poll_timeout`   | `duration`    | Timeout when polling the URL.           | `"10s"` | no

## Example

This example imports custom components from an HTTP response and instantiates a custom component for adding two numbers:

{{< collapse title="HTTP response" >}}
```river
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```
{{< /collapse >}}

{{< collapse title="importer.river" >}}
```river
import.http "math" {
  url = SERVER_URL
}

math.add "default" {
  a = 15
  b = 45
}
```
{{< /collapse >}}
