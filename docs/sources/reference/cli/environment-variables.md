---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/environment-variables/
description: Learn about the environment variables you can use with Alloy
menuTitle: Environment variables
title: Environment variables
weight: 500
---

# Environment variables

You can use environment variables to control the run-time behavior of {{< param "FULL_PRODUCT_NAME" >}}.

## Usage

Usage:

```shell
alloy run [<FLAG> ...] <PATH_NAME> <ENVIRONMENT_VARIABLE>
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the input and output of the command.
* _`<PATH_NAME>`_: Required. The {{< param "PRODUCT_NAME" >}} configuration file/directory path.
* _`<ENVIRONMENT_VARIABLE>`_: One or more environment variables.

The following environment variables are supported:

* `GODEBUG`
* `HTTP_PROXY`

Refer to the [Go runtime][runtime] documentation for more information about Go runtime environment variables.

## Arguments

The following arguments are supported.

Environment variable | Argument                   | Description                                                                     | Default
---------------------|----------------------------|---------------------------------------------------------------------------------|--------
`GODEBUG`            | `x509usefallbackroots`     | Enforce a fallback on the X.509 trusted root certificates. Set to `1` to enable | `0`
`HTTP_PROXY`         | `http://proxy.example.com` | The hostname or IP address of the proxy server.                                 |

## Example

The following example shows how to use the GODEBUG environment variable to enforce a fallback on the X.509 trusted root certificates.

```alloy
alloy run GODEBUG=x509usefallbackroots=1
```

[runtime]: https://pkg.go.dev/runtime
