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
* `PPROF_MUTEX_PROFILING_PERCENT`
* `PPROF_BLOCK_PROFILING_RATE`
* `GOMEMLIMIT`


Refer to the [Go runtime][runtime] documentation for more information about Go runtime environment variables.

## GODEBUG

You can use the `GODEBUG` environment variable to control the debugging variables within the Go runtime.  The following arguments are supported.

 Argument               | Description                                                                                          | Default 
------------------------|------------------------------------------------------------------------------------------------------|---------
 `x509usefallbackroots` | Enforce a fallback on the X.509 trusted root certificates. Set to `1` to enable.                     | `0`     
 `netdns`               | Force a resolver. Set to `go` for a pure Go resolver. Set to `cgo` or `win32` for a native resolver. |
 `netdns`               | Show resolver debugging information. Set to `1` for basic information. Set to `2` for verbose.       |

## HTTP_PROXY

You can use the `HTTP_PROXY` environment variable to define the hostname or IP address of the proxy server.  For example, you can set the proxy to `http://proxy.example.com`.

## PPROF_MUTEX_PROFILING_PERCENT

You can use the `PPROF_MUTEX_PROFILING_PERCENT` environment variable to define the percentage of mutex profiles to retrieve from the pprof mutex endpoint. If you set this variable to `5`, then 5 percent of the mutexes are sampled. The default value is `0.01`.

## PPROF_BLOCK_PROFILING_RATE

You can use the `PPROF_BLOCK_PROFILING_RATE` environment variable to define the rate that mutexes are tracked. You can use the following values with this environment variable. The default value is `10000`.

* `0`: Nothing is tracked.
* `1`: All mutexes are tracked.
* A value greater than `1`: The number of nanoseconds to track mutexes.

### GOMEMLIMIT

Usually, the [Go runtime][runtime] will release memory back to the operating system when requested.
In some environments, this may cause issues such as Out Of Memory (OOM) errors.
You can use the `GOMEMLIMIT` environment variable to set a soft memory cap and limit the maximum memory {{< param "PRODUCT_NAME" >}} can use.
You can set `GOMEMLIMIT` to a numeric value in bytes with an optional unit suffix.
The supported unit suffixes are `B`, `KiB`, `MiB`, `GiB`, and `TiB`.
Don't treat the `GOMEMLIMIT` environment variable as a hard memory limit.
{{< param "PRODUCT_NAME" >}}  processes can use more memory if that memory is required.
A rough number is to set `GOMEMLIMIT` to is 90% of the maximum memory required.
For example, if you want to keep memory usage below `10GiB`, use `GOMEMLIMIT=9GiB`.

#### Automatically set GOMEMLIMIT

The `GOMEMLIMIT` environment variable is either automatically set to 90% of an available `cgroup` value, or you can explicitly set the  `GOMEMLIMIT` environment variable before you run  {{< param "PRODUCT_NAME" >}}.
No changes will occur if the limit cannot be determined and you did not explicitly define a  `GOMEMLIMIT` value.

[runtime]: https://pkg.go.dev/runtime
