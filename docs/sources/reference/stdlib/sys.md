---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/sys/
description: Learn about sys functions
aliases:
  - ./env/ # /docs/alloy/latest/reference/stdlib/env/
menuTitle: sys
title: sys
---

# sys

The `sys` namespace contains functions related to the system.

## sys.env

The `sys.env` function gets the value of an environment variable from the system {{< param "PRODUCT_NAME" >}} is running on.
If the environment variable doesn't exist, `sys.env` returns an empty string.

### Examples

```alloy
> sys.env("HOME")
"/home/alloy"

> sys.env("DOES_NOT_EXIST")
""
```
