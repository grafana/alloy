---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-compression-field/
description: Shared content, otelcol compression field
headless: true
---

By default, requests are compressed with Gzip.
The `compression` argument controls which compression mechanism to use. Supported strings are:

* `"gzip"`
* `"zlib"`
* `"deflate"`
* `"snappy"`
* `"zstd"`

If you set `compression` to `"none"` or an empty string `""`, the requests aren't compressed.
