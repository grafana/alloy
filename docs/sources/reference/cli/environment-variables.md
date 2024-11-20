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
* `HTTPS_PROXY`
* `NO_PROXY`
* `PPROF_MUTEX_PROFILING_PERCENT`
* `PPROF_BLOCK_PROFILING_RATE`
* `GOMEMLIMIT`
* `AUTOMEMLIMIT`
* `GOGC`
* `GOMAXPROCS`
* `GOTRACEBACK`

Refer to the [Go runtime][runtime] documentation for more information about Go runtime environment variables.

## GODEBUG

You can use the `GODEBUG` environment variable to control the debugging variables within the Go runtime.  The following arguments are supported.

 Argument               | Description                                                                                          | Default 
------------------------|------------------------------------------------------------------------------------------------------|---------
 `x509usefallbackroots` | Enforce a fallback on the X.509 trusted root certificates. Set to `1` to enable.                     | `0`     
 `netdns`               | Force a resolver. Set to `go` for a pure Go resolver. Set to `cgo` or `win32` for a native resolver. |
 `netdns`               | Show resolver debugging information. Set to `1` for basic information. Set to `2` for verbose.       |

## HTTP_PROXY, HTTPS_PROXY, NO_PROXY

You can use the `HTTP_PROXY` environment variable to define the hostname or IP address of the proxy server for HTTP requests.  For example, you can set the proxy to `http://proxy.example.com`.

You can use the `HTTPS_PROXY` environment variable to define the proxy server for HTTPS requests in the same manner as `HTTP_PROXY`.

The `NO_PROXY` environment variable is used to define any hosts that should be excluded from proxying. `NO_PROXY` should contain a comma delimited list of any of the following options.

 Option                 | Description                                                                                                    | Examples
------------------------|----------------------------------------------------------------------------------------------------------------|---------
 IP Address             | A single IP address (with optional port)                                                                       | `1.2.3.4` or `1.2.3.4:80`
 CIDR Block             | A group of IP addresses that share a network prefix.                                                           | `1.2.3.4/8`
 Domain                 | A domain name matches that name and all subdomains. A domain name with a leading "." matches subdomains only.  | `example.com` or `.example.com`
 Asterisk               | A single asterisk indicates that no proxying should be done.                                                   | `*`

## PPROF_MUTEX_PROFILING_PERCENT

You can use the `PPROF_MUTEX_PROFILING_PERCENT` environment variable to define the percentage of mutex profiles to retrieve from the pprof mutex endpoint. If you set this variable to `5`, then 5 percent of the mutexes are sampled. The default value is `0.01`.

## PPROF_BLOCK_PROFILING_RATE

You can use the `PPROF_BLOCK_PROFILING_RATE` environment variable to define the rate that mutexes are tracked. You can use the following values with this environment variable. The default value is `10000`.

* `0`: Nothing is tracked.
* `1`: All mutexes are tracked.
* A value greater than `1`: The number of nanoseconds to track mutexes.

## GOMEMLIMIT

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

The `GOMEMLIMIT` environment variable is either automatically set to 90% of an available `cgroup` value using the [automemlimit] module, or you can explicitly set the  `GOMEMLIMIT` environment variable before you run  {{< param "PRODUCT_NAME" >}}.
You can also change the 90% ratio by setting the `AUTOMEMLIMIT` environment variable to a float value between `0` and `1.0`.
No changes will occur if the limit cannot be determined and you did not explicitly define a  `GOMEMLIMIT` value.

## GOGC

The `GOGC` environment variable controls the mechanism that triggers Go's garbage collection.
It represents the garbage collection target percentage. A collection is triggered when the ratio 
of freshly allocated data to live data remaining after the previous collection reaches this percentage.
The default value if this variable is not provided is 100. Setting GOGC=off can also disable garbage collection entirely.

Configuring this value in conjunction with `GOMEMLIMIT` can help in situations where {{< param "PRODUCT_NAME" >}} 
is consuming too much memory. Go provides a [very in-depth guide][gc_guide] to understanding `GOGC` and `GOMEMLIMIT`.

## GOMAXPROCS

The `GOMAXPROCS` environment variable defines the limit of OS threads that can simultaneously execute user-level Go code.
This limit does not affect the number of threads that can be blocked in system calls on behalf of Go code and those
threads are not counted against `GOMAXPROCS`.

## GOTRACEBACK

The `GOTRACEBACK` environment variable defines the behavior of the Go panic output. The standard panic output behavior is usually
sufficient to debug and resolve an issue, but this setting can be used in cases of a reproducable panic to collect
additional information from the runtime if required. The following values are supported.

Value            | Description                                                                     | Traces include runtime internal functions
-----------------|---------------------------------------------------------------------------------|------------------------------------------
 `none` or `0`   | Omit goroutine stack traces entirely from the panic output.                     | -
 `single`        | Print the stack trace for the current goroutine.                                | No
 `all` or `1`    | Print the stack traces for all user created goroutines.                         | No
 `system` or `2` | Print the stack traces for all user created and runtime created goroutines.     | Yes
 `crash`         | Similar to `system`, but also triggers OS specific additional behavior. For example, on Unix systems this raises a SIGABRT to trigger a code dump.   | Yes
 `wer`           | Similar to `crash`, but does not disable Windows Error Reporting.               | Yes

[runtime]: https://pkg.go.dev/runtime
[automemlimit]: https://github.com/KimMachineGun/automemlimit
[gc_guide]: https://tip.golang.org/doc/gc-guide#GOGC
[Windows]: ../../../configure/windows