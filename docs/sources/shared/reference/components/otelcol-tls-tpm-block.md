---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/otelcol-tls-client-block/
description: Shared content, otelcol tls client block
headless: true
---

The following arguments are supported:

| Name         | Type     | Description                                                        | Default | Required |
|--------------|----------|--------------------------------------------------------------------|---------|----------|
| `auth`       | `string` | The authorization value used to authenticate the TPM device.       | `""`    | no       |
| `enabled`    | `bool`   | Load the `tls.key_file` from TPM.                                  | `false` | no       |
| `owner_auth` | `string` | The owner authorization value used to authenticate the TPM device. | `""`    | no       |
| `path`       | `string` | Path to the TPM device or Unix domain socket.                      | `""`    | no       |

The [trusted platform module][tpm] (TPM) configuration can be used for loading TLS key from TPM. Currently only TSS2 format is supported.

The `path` attribute is not supported on Windows.

In the example below, the private key `my-tss2-key.key` in TSS2 format will be loaded from the TPM device `/dev/tmprm0`:

```alloy
otelcol.example.component "<LABEL>" {
    ...
    tls {
        ...
        key_file = "my-tss2-key.key"
        tpm {
            enabled = true
            path = "/dev/tpmrm0"
        }
    }
}
```

[tpm]: https://trustedcomputinggroup.org/resource/trusted-platform-module-tpm-summary/