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
* `GOMEMLIMIT`

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

### GOMEMLIMIT

Normally the [Go runtime][runtime] will release memory back to the Operating System when requested. In certain environments this may cause issues such as Out of Memory errors. The `GOMEMLIMIT` variable is a numeric value in bytes with an optional unit suffix. The supported suffixes include B, KiB, MiB, GiB, and TiB. This should not be treated has a hard limit. The process can use more memory if that memory is required. A rough number is to set `GOMEMLIMIT` to 90% of the maximum amount of memory usage. For example if you wanted to keep memory usage below `10GiB` then use `GOMEMLIMIT=8GiB`.

#### Automatic setting of GOMEMLIMIT

The `GOMEMLIMIT` variable will be automatically set if {{< param "PRODUCT_NAME" >}} can determine the appropriate value. If the `GOMEMLIMIT` value is set before running  {{< param "PRODUCT_NAME" >}} then that setting will be honored. The value will be set to 90% of the cgroup value set. If the limit cannot be determined and no value was explicitly passed then no changes will occur.

[runtime]: https://pkg.go.dev/runtime
