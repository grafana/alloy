---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/environment-variables/
description: Learn about the environment variables you can use with Alloy
menuTitle: Environment variables
title: Environment variables
weight: 500
---

# Environment variables

You can use environment variables to control the run-time behavior of {{< param "FULL_PRODUCT_NAME" >}}.

The following environment variables are supported:

* `GODEBUG`
* `HTTP_PROXY`

Refer to the [Go runtime][runtime] documentation for more information about Go runtime environment variables.

## Arguments

The following arguments are supported.

### GODEBUG

Argument                        | Description                                                                                          | Default
--------------------------------|------------------------------------------------------------------------------------------------------|--------
`x509usefallbackroots`          | Enforce a fallback on the X.509 trusted root certificates. Set to `1` to enable.                     | `0`
`netdns`                        | Force a resolver. Set to `go` for a pure Go resolver. Set to `cgo` or `win32` for a native resolver. |
`netdns`                        | Show resolver debugging information. Set to `1` for basic information. Set to `2` for verbose.       |

### HTTP_PROXY

Argument                   | Description                                     | Default
---------------------------|-------------------------------------------------|--------
`http://proxy.example.com` | The hostname or IP address of the proxy server. |

[runtime]: https://pkg.go.dev/runtime
