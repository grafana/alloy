---
canonical: https://grafana.com/docs/alloy/latest/tutorials/first-components-and-stdlib/
description: Learn the basics of the Grafana Alloy configuration syntax
menuTitle: First components and the standard library
title: First components and the standard library in Grafana Alloy
weight: 200
---

# First components and the standard library in {{% param "FULL_PRODUCT_NAME" %}}

This tutorial covers the basics of the {{< param "PRODUCT_NAME" >}} configuration syntax and the standard library.
It introduces a basic pipeline that collects metrics from the host and sends them to Prometheus.

## Before you begin

To complete this tutorial:

* You must set up a [local Grafana instance][previous tutorial].

### Recommended reading

- [{{< param "PRODUCT_NAME" >}} configuration syntax][configuration syntax]

## {{% param "PRODUCT_NAME" %}} configuration syntax basics

An {{< param "PRODUCT_NAME" >}} configuration file contains three elements:

1. **Attributes**

   `key = value` pairs used to configure individual settings.

    ```alloy
    url = "http://localhost:9090"
    ```

1. **Expressions**

   Expressions are used to compute values.
   They can be constant values (for example, `"localhost:9090"`), or they can be more complex (for example, referencing a component's export: `prometheus.exporter.unix.targets`.
   They can also be a mathematical expression: `(1 + 2) * 3`, or a standard library function call: `env("HOME")`). We will use more expressions as we go along the examples.
   If you are curious, you can find a list of available standard library functions in the [Standard library documentation][].

1. **Blocks**

   Blocks are used to configure components with groups of attributes or nested blocks.
   The following example block can be used to configure the logging output of {{< param "PRODUCT_NAME" >}}:

    ```alloy
    logging {
        level  = "debug"
        format = "json"
    }
    ```

    {{< admonition type="note" >}}
    The default log level is `info` and the default log format is `logfmt`.
    {{< /admonition >}}

    Try pasting this into `config.alloy` and running `<BINARY_FILE_PATH> run config.alloy` to see what happens. Replace _`<BINARY_FILE_PATH>`_ with the path to the {{< param "PRODUCT_NAME" >}} binary.

    Congratulations, you've just written your first {{< param "PRODUCT_NAME" >}} configuration file.
    This configuration won't do anything, so let's add some components to it.

    {{< admonition type="note" >}}
    Comments in {{< param "PRODUCT_NAME" >}} syntax are prefixed with `//` and are single-line only. For example: `// This is a comment`.
    {{< /admonition >}}

## Components

Components are the building blocks of an {{< param "PRODUCT_NAME" >}} configuration. They are configured and linked to create pipelines that collect, process, and output your telemetry data. Components are configured with `Arguments` and have `Exports` that may be referenced by other components.

### Recommended reading

- [Components][]
- [Components configuration language][]
- [Component controller][]

### An example pipeline

Look at the following simple pipeline:

```alloy
local.file "example" {
    filename = env("HOME") + "/file.txt"
}

prometheus.remote_write "local_prom" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"

        basic_auth {
            username = "admin"
            password = local.file.example.content
        }
    }
}
```

{{< admonition type="note" >}}
A list of all available components can be found in the [Component reference][].
Each component has a link to its documentation, which contains a description of what the component does, its arguments, its exports, and examples.

[Component reference]: ../../reference/components/
{{< /admonition >}}

This pipeline has two components: `local.file` and `prometheus.remote_write`.
The `local.file` component is configured with a single argument, `filename`, which is set by calling the [env][] standard library function to retrieve the value of the `HOME` environment variable and concatenating it with the string `"file.txt"`.
The `local.file` component has a single export, `content`, which contains the contents of the file.

The `prometheus.remote_write` component is configured with an `endpoint` block, containing the `url` attribute and a `basic_auth` block.
The `url` attribute is set to the URL of the Prometheus remote write endpoint.
The `basic_auth` block contains the `username` and `password` attributes, which are set to the string `"admin"` and the `content` export of the `local.file` component, respectively.
The `content` export is referenced by using the syntax `local.file.example.content`, where `local.file.example` is the fully qualified name of the component (the component's type + its label) and `content` is the name of the export.

{{< figure src="/media/docs/alloy/diagram-example-basic-alloy.png" width="600" alt="Example pipeline with local.file and prometheus.remote_write components" >}}

{{< admonition type="note" >}}
The `local.file` component's label is set to `"example"`, so the fully qualified name of the component is `local.file.example`.
The `prometheus.remote_write` component's label is set to `"local_prom"`, so the fully qualified name of the component is `prometheus.remote_write.local_prom`.
{{< /admonition >}}

This example pipeline still doesn't do anything, so its time to add some more components to it.

## Shipping your first metrics

Now that you have a simple pipeline, you can ship your first metrics.

### Recommended reading

- Optional: [prometheus.exporter.unix][]
- Optional: [prometheus.scrape][]
- Optional: [prometheus.remote_write][]

### Modify your pipeline and scrape the metrics

Make a simple pipeline with a `prometheus.exporter.unix` component, a `prometheus.scrape` component to scrape it, and a `prometheus.remote_write` component to send the scraped metrics to Prometheus.

```alloy
prometheus.exporter.unix "localhost" {
    // This component exposes a lot of metrics by default, so we will keep all of the default arguments.
}

prometheus.scrape "default" {
    // Setting the scrape interval lower to make it faster to be able to see the metrics
    scrape_interval = "10s"

    targets    = prometheus.exporter.unix.localhost.targets
    forward_to = [
        prometheus.remote_write.local_prom.receiver,
    ]
}

prometheus.remote_write "local_prom" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"
    }
}
```

Run {{< param "PRODUCT_NAME" >}} with the following command:

```bash
<BINARY_FILE_PATH> run config.alloy
```

Replace the following:

* _`<BINARY_FILE_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary.

Navigate to [http://localhost:3000/explore][] in your browser.
After ~15-20 seconds, you should be able to see the metrics from the `prometheus.exporter.unix` component.
Try querying for `node_memory_Active_bytes` to see the active memory of your host.

<p align="center">
<img src="/media/docs/alloy/screenshot-memory-usage.png" alt="Screenshot of node_memory_Active_bytes query in Grafana" />
</p>

## Visualize the relationship between components

The following diagram is an example pipeline:

{{< figure src="/media/docs/alloy/diagram-example-pipeline-prometheus.scrape-alloy.png" width="600" alt="Example pipeline with a prometheus.scrape, prometheus.exporter.unix, and prometheus.remote_write components" >}}

Your pipeline configuration defines three components:

- `prometheus.scrape` - A component that scrapes metrics from components that export targets.
- `prometheus.exporter.unix` - A component that exports metrics from the host, built around [node_exporter][].
- `prometheus.remote_write` - A component that sends metrics to a Prometheus remote-write compatible endpoint.

The `prometheus.scrape` component references the `prometheus.exporter.unix` component's targets export, which is a list of scrape targets.
The `prometheus.scrape` component forwards the scraped metrics to the `prometheus.remote_write` component.

One rule is that components can't form a cycle.
This means that a component can't reference itself directly or indirectly.
This is to prevent infinite loops from forming in the pipeline.

## Exercise

The following exercise guides you through modifying your pipeline to scrape metrics from Redis.

### Recommended Reading

- Optional: [prometheus.exporter.redis][]

Start a container running Redis and configure {{< param "PRODUCT_NAME" >}} to scrape the metrics.

```bash
docker container run -d --name alloy-redis -p 6379:6379 --rm redis
```

Try modifying the pipeline to scrape metrics from the Redis exporter.
You can refer to the [prometheus.exporter.redis][] component documentation for more information on how to configure it.

To give a visual hint, you want to create a pipeline that looks like this:

{{< figure src="/media/docs/alloy/diagram-example-pipeline-exercise-alloy.png" alt="Exercise pipeline, with a scrape, unix_exporter, redis_exporter, and remote_write component" >}}

{{< admonition type="tip" >}}
Refer to the [concat][] standard library function for information about combining lists of values into a single list.

[concat]: ../../reference/stdlib/concat/
{{< /admonition >}}

You can run {{< param "PRODUCT_NAME" >}} with the new configuration file with the following command:

```bash
<BINARY_FILE_PATH> run config.alloy
```

Replace the following:

* _`<BINARY_FILE_PATH>`_: The path to the {{< param "PRODUCT_NAME" >}} binary.

Navigate to [http://localhost:3000/explore][] in your browser.
After the first scrape, you should be able to query for `redis` metrics as well as `node` metrics.

To shut down the Redis container, run:

```bash
docker container stop alloy-redis
```

If you get stuck, you can always view a solution here:

{{< collapse title="Solution" >}}

```alloy
// Configure your first components, learn about the standard library, and learn how to run Grafana Alloy

// prometheus.exporter.redis collects information about Redis and exposes
// targets for other components to use
prometheus.exporter.redis "local_redis" {
    redis_addr = "localhost:6379"
}

prometheus.exporter.unix "localhost" { }

// prometheus.scrape scrapes the targets that it is configured with and forwards
// the metrics to other components (typically prometheus.relabel or prometheus.remote_write)
prometheus.scrape "default" {
    // This is scraping too often for typical use-cases, but is easier for testing and demo-ing!
    scrape_interval = "10s"

    // Here, prometheus.exporter.redis.local_redis.targets refers to the 'targets' export
    // of the prometheus.exporter.redis component with the label "local_redis".
    //
    // If you have more than one set of targets that you would like to scrape, you can use
    // the 'concat' function from the standard library to combine them.
    targets    = concat(prometheus.exporter.redis.local_redis.targets, prometheus.exporter.unix.localhost.targets)
    forward_to = [prometheus.remote_write.local_prom.receiver]
}

// prometheus.remote_write exports a 'receiver', which other components can forward
// metrics to and it will remote_write them to the configured endpoint(s)
prometheus.remote_write "local_prom" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"
    }
}

```

{{< /collapse >}}

## Finishing up and next steps

You might have noticed that running {{< param "PRODUCT_NAME" >}} with the configurations created a directory called `data-alloy` in the directory you ran {{< param "PRODUCT_NAME" >}} from.
This directory is where components can store data, such as the `prometheus.exporter.unix` component storing its WAL (Write Ahead Log).
If you look in the directory, do you notice anything interesting? The directory for each component is the fully qualified name.

If you'd like to store the data elsewhere, you can specify a different directory by supplying the `--storage.path` flag to {{< param "PRODUCT_NAME" >}}'s run command, for example, `<BINARY_FILE_PATH> run config.alloy --storage.path /etc/alloy`. Replace _`<BINARY_FILE_PATH>`_ with the path to the {{< param "PRODUCT_NAME" >}} binary.
Generally, you can use a persistent directory for this, as some components may use the data stored in this directory to perform their function.

In the next tutorial, you learn how to configure {{< param "PRODUCT_NAME" >}} to collect logs from a file and send them to Loki.
You also learn how to use different components to process metrics and logs.

[previous tutorial]: ../send-logs-to-loki/#set-up-a-local-grafana-instance
[configuration syntax]: ../../get-started/configuration-syntax/
[Standard library documentation]: ../../reference/stdlib/
[node_exporter]: https://github.com/prometheus/node_exporter
[prometheus.exporter.redis]: ../../reference/components/prometheus/prometheus.exporter.redis/
[prometheus.exporter.unix]: ../../reference/components/prometheus/prometheus.exporter.unix/
[prometheus.scrape]: ../../reference/components/prometheus/prometheus.scrape/
[prometheus.remote_write]: ../../reference/components/prometheus/prometheus.remote_write/
[Components]: ../../get-started/components/
[Component controller]: ../../get-started/component_controller/
[Components configuration language]: ../../get-started/configuration-syntax/components/
[env]: ../../reference/stdlib/env/
