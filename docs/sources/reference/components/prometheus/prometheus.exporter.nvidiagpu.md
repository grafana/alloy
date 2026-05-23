---
aliases:
- /docs/alloy/latest/reference/components/prometheus/prometheus.exporter.nvidiagpu/
- /docs/alloy/latest/reference/components/prometheus.exporter.nvidiagpu/
title: prometheus.exporter.nvidiagpu
---

# prometheus.exporter.nvidiagpu

`prometheus.exporter.nvidiagpu` embeds the [`nvidia_gpu_exporter`](https://github.com/utkuozdemir/nvidia_gpu_exporter) to export metrics from NVIDIA GPUs.

## Usage

```alloy
prometheus.exporter.nvidiagpu "<LABEL>" {
}
```

## Arguments

The following arguments are supported:

Name | Type | Description | Default | Required
---- | ---- | ----------- | ------- | --------
`nvidia_smi_command` | `string` | The command to run to invoke `nvidia-smi`. | `"nvidia-smi"` | no

## Exported fields

The following fields are exported and can be referenced by other components:

Name | Type | Description
---- | ---- | -----------
`targets` | `list(map(string))` | The targets that can be used to collect metrics from the exporter.

## Component health

`prometheus.exporter.nvidiagpu` is only reported as unhealthy if given an invalid configuration.
In those cases, exported fields retain their last healthy values.

## Debug information

`prometheus.exporter.nvidiagpu` does not expose any component-specific debug information.

## Debug metrics

`prometheus.exporter.nvidiagpu` does not expose any component-specific debug metrics.

## Example

This example uses a [`prometheus.scrape`][] component to collect metrics from `prometheus.exporter.nvidiagpu`:

```alloy
prometheus.exporter.nvidiagpu "example" {
  nvidia_smi_command = "nvidia-smi"
}

// Configure a prometheus.scrape component to collect nvidia gpu metrics.
prometheus.scrape "demo" {
  targets    = prometheus.exporter.nvidiagpu.example.targets
  forward_to = [prometheus.remote_write.demo.receiver]
}

prometheus.remote_write "demo" {
  endpoint {
    url = "<PROMETHEUS_REMOTE_WRITE_URL>"
  }
}
```

[`prometheus.scrape`]: ../prometheus.scrape/
