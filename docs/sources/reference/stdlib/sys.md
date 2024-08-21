---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/sys/
description: Learn about sys functions
title: sys
---

# sys.env (previously `env`)

The `sys.env` function gets the value of an environment variable from the system {{< param "PRODUCT_NAME" >}} is running on.
If the environment variable does not exist, `sys.env` returns an empty string.

## Examples

```
> sys.env("HOME")
"/home/alloy"

> sys.env("DOES_NOT_EXIST")
""
```
