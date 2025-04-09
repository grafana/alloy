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

## Blocks

The following blocks are supported inside the definition of `import.http`:

Hierarchy                    | Block             | Description                                              | Required
-----------------------------|-------------------|----------------------------------------------------------|---------
client                       | [client][]        | HTTP client settings when connecting to the endpoint.    | no
client > basic_auth          | [basic_auth][]    | Configure basic_auth for authenticating to the endpoint. | no
client > authorization       | [authorization][] | Configure generic authorization to the endpoint.         | no
client > oauth2              | [oauth2][]        | Configure OAuth2 for authenticating to the endpoint.     | no
client > oauth2 > tls_config | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no
client > tls_config          | [tls_config][]    | Configure TLS settings for connecting to the endpoint.   | no

The `>` symbol indicates deeper levels of nesting.
For example, `client > basic_auth` refers to an `basic_auth` block defined inside a `client` block.

### client block

The `client` block configures settings used to connect to the HTTP server.

{{< docs/shared lookup="reference/components/http-client-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### basic_auth block

The `basic_auth` block configures basic authentication to use when polling the configured URL.

{{< docs/shared lookup="reference/components/basic-auth-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### authorization block

The `authorization` block configures custom authorization to use when polling the configured URL.

{{< docs/shared lookup="reference/components/authorization-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### oauth2 block

The `oauth2` block configures OAuth2 authorization to use when polling the configured URL.

{{< docs/shared lookup="reference/components/oauth2-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

### tls_config block

The `tls_config` block configures TLS settings for connecting to HTTPS servers.

{{< docs/shared lookup="reference/components/tls-config-block.md" source="alloy" version="<ALLOY_VERSION>" >}}

## Example

This example imports custom components from an HTTP response and instantiates a custom component for adding two numbers:

module.alloy

```alloy
declare "add" {
  argument "a" {}
  argument "b" {}

  export "sum" {
    value = argument.a.value + argument.b.value
  }
}
```

main.alloy

```alloy
import.http "math" {
  url = SERVER_URL
}

math.add "default" {
  a = 15
  b = 45
}
```

[client]: #client-block
[basic_auth]: #basic_auth-block
[authorization]: #authorization-block
[oauth2]: #oauth2-block
[tls_config]: #tls_config-block
