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

Usually, the [Go runtime][runtime] will release memory back to the operating system when requested.
In some environments, this may cause issues such as Out Of Memory (OOM) errors.
You can use the `GOMEMLIMIT` environment variable to set a soft memory cap and limit the maximum memory {{< param "PRODUCT_NAME" >}} can use.
You can set `GOMEMLIMIT` to a numeric value in bytes with an optional unit suffix.
The supported unit suffixes are `B`, `KiB`, `MiB`, `GiB`, and `TiB`.
Don't treat the `GOMEMLIMIT` environment variable as a hard memory limit.
{{< param "PRODUCT_NAME" >}}  processes can use more memory if that memory is required.
A rough number is to set `GOMEMLIMIT` to is 90% of the maximum memory required.
For example, if you want to keep memory usage below `10GiB`, use `GOMEMLIMIT=8GiB`.

#### Automatic setting of GOMEMLIMIT

The `GOMEMLIMIT` environment variable is set automatically {{< param "PRODUCT_NAME" >}} can determine the appropriate value.
If you set the `GOMEMLIMIT` environment variable before you run  {{< param "PRODUCT_NAME" >}} then that setting will be used.
The variable value is set to 90% of the `cgroup` value set.
No changes will occur if the limit cannot be determined and you did not explicitly define a  `GOMEMLIMIT` value.

[runtime]: https://pkg.go.dev/runtime
