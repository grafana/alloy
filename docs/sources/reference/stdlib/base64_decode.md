---
canonical: https://grafana.com/docs/alloy/latest/reference/stdlib/base64_decode/
description: Learn about base64_decode
title: base64_decode
---

# base64_decode

The `base64_decode` function decodes a RFC4648-compliant Base64-encoded string 
into the original string. 

`base64_decode` fails if the provided string argument contains invalid Base64 data. 

## Examples

```
> base64_decode("dGFuZ2VyaW5l")
tangerine
```
