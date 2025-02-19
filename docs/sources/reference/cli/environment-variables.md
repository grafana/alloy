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

- `GODEBUG`
- `HTTP_PROXY`
- `PPROF_MUTEX_PROFILING_PERCENT`
- `PPROF_BLOCK_PROFILING_RATE`

Refer to the [Go runtime][runtime] documentation for more information about Go runtime environment variables.

## GODEBUG

You can use the `GODEBUG` environment variable to control the debugging variables within the Go runtime. The following arguments are supported.

| Argument               | Description                                                                                          | Default |
| ---------------------- | ---------------------------------------------------------------------------------------------------- | ------- |
| `x509usefallbackroots` | Enforce a fallback on the X.509 trusted root certificates. Set to `1` to enable.                     | `0`     |
| `netdns`               | Force a resolver. Set to `go` for a pure Go resolver. Set to `cgo` or `win32` for a native resolver. |
| `netdns`               | Show resolver debugging information. Set to `1` for basic information. Set to `2` for verbose.       |

## HTTP_PROXY

You can use the `HTTP_PROXY` environment variable to define the hostname or IP address of the proxy server. For example, you can set the proxy to `http://proxy.example.com`.

## PPROF_MUTEX_PROFILING_PERCENT

You can use the `PPROF_MUTEX_PROFILING_PERCENT` environment variable to define the percentage of mutex profiles to retrieve from the pprof mutex endpoint. If you set this variable to `5`, then 5 percent of the mutexes are sampled. The default value is `0.01`.

## PPROF_BLOCK_PROFILING_RATE

You can use the `PPROF_BLOCK_PROFILING_RATE` environment variable to define the rate that mutexes are tracked. You can use the following values with this environment variable. The default value is `10000`.

- `0`: Nothing is tracked.
- `1`: All mutexes are tracked.
- A value greater than `1`: The number of nanoseconds to track mutexes.

[runtime]: https://pkg.go.dev/runtime
