---
canonical: https://grafana.com/docs/alloy/latest/reference/config-blocks/import.http/
description: Learn about the import.http configuration block
title: import.http
---

# import.http

`import.http` retrieves a module from an HTTP server.

## Usage

```alloy
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
```alloy
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```
{{< /collapse >}}

{{< collapse title="importer.alloy" >}}
```alloy
import.http "math" {
  url = SERVER_URL
}

math.add "default" {
  a = 15
  b = 45
}
```
{{< /collapse >}}
