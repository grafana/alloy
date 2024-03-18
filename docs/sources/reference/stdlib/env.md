---
aliases:
- ./reference/stdlib/env/
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/env/
description: Learn about env
title: env
---

# env

The `env` function gets the value of an environment variable from the system {{< param "PRODUCT_NAME" >}} is running on.
If the environment variable does not exist, `env` returns an empty string.

## Examples

```
> env("HOME")
"/home/grafana-alloy"

> env("DOES_NOT_EXIST")
""
```
