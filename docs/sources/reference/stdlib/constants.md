---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/constants/
description: Learn about constants
title: constants
---

# constants

The `constants` object exposes a list of constant values about the system {{< param "PRODUCT_NAME" >}} is running on:

* `constants.hostname`: The hostname of the machine {{< param "PRODUCT_NAME" >}} is running   on.
* `constants.os`: The operating system {{< param "PRODUCT_NAME" >}} is running on.
* `constants.arch`: The architecture of the system {{< param "PRODUCT_NAME" >}} is running on.

## Examples

```alloy
> constants.hostname
"my-hostname"

> constants.os
"linux"

> constants.arch
"amd64"
```
