---
aliases:
- /docs/grafana-cloud/agent/flow/tutorials/flow-by-example/first-components-and-stdlib/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tutorials/flow-by-example/first-components-and-stdlib/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tutorials/flow-by-example/first-components-and-stdlib/
- /docs/grafana-cloud/send-data/agent/flow/tutorials/first-components-and-stdlib/
canonical: https://grafana.com/docs/agent/latest/flow/tutorials/flow-by-example/first-components-and-stdlib/
description: Learn about the basics of River and the configuration language
title: First components and introducing the standard library
weight: 20
---

# First components and the standard library

This tutorial covers the basics of the River language and the standard library. It introduces a basic pipeline that collects metrics from the host and sends them to Prometheus.

## River basics

[Configuration language]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/config-language/
[Configuration language concepts]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/configuration_language/
[Standard library documentation]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/stdlib/

**Recommended reading**

- [Configuration language][]
- [Configuration language concepts][]

[River](https://github.com/grafana/river) is an HCL-inspired configuration language used to configure {{< param "PRODUCT_NAME" >}}. A River file is comprised of three things:

1. **Attributes**

   `key = value` pairs used to configure individual settings.

    ```river
    url = "http://localhost:9090"
    ```

1. **Expressions**

   Expressions are used to compute values. They can be constant values (for example, `"localhost:9090"`), or they can be more complex (for example, referencing a component's export: `prometheus.exporter.unix.targets`. They can also be a mathematical expression: `(1 + 2) * 3`, or a standard library function call: `env("HOME")`). We will use more expressions as we go along the examples. If you are curious, you can find a list of available standard library functions in the [Standard library documentation][].

1. **Blocks**

   Blocks are used to configure components with groups of attributes or nested blocks. The following example block can be used to configure the logging output of {{< param "PRODUCT_NAME" >}}:

    ```river
    logging {
        level  = "debug"
        format = "json"
    }
    ```

    {{< admonition type="note" >}}
The default log level is `info` and the default log format is `logfmt`.
    {{< /admonition >}}

    Try pasting this into `config.river` and running `/path/to/agent run config.river` to see what happens.

    Congratulations, you've just written your first River file! You've also just written your first {{< param "PRODUCT_NAME" >}} configuration file. This configuration won't do anything, so let's add some components to it.

    {{< admonition type="note" >}}
Comments in River are prefixed with `//` and are single-line only. For example: `// This is a comment`.
    {{< /admonition >}}

## Components

[Components]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/components/
[Component controller]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/component_controller/
[Components configuration language]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/concepts/config-language/components/
[env]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/stdlib/env/

**Recommended reading**

- [Components][]
- [Components configuration language][]
- [Component controller][]

Components are the building blocks of a {{< param "PRODUCT_NAME" >}} configuration. They are configured and linked to create pipelines that collect, process, and output your telemetry data. Components are configured with `Arguments` and have `Exports` that may be referenced by other components.

Let's look at a simple example pipeline:

```river
local.file "example" {
    path = env("HOME") + "file.txt"
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
[Component reference]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/

A list of all available components can be found in the [Component reference][]. Each component has a link to its documentation, which contains a description of what the component does, its arguments, its exports, and examples.
{{< /admonition >}}

This pipeline has two components: `local.file` and `prometheus.remote_write`. The `local.file` component is configured with a single argument, `path`, which is set by calling the [env][] standard library function to retrieve the value of the `HOME` environment variable and concatenating it with the string `"file.txt"`. The `local.file` component has a single export, `content`, which contains the contents of the file.

The `prometheus.remote_write` component is configured with an `endpoint` block, containing the `url` attribute and a `basic_auth` block. The `url` attribute is set to the URL of the Prometheus remote write endpoint. The `basic_auth` block contains the `username` and `password` attributes, which are set to the string `"admin"` and the `content` export of the `local.file` component, respectively. The `content` export is referenced by using the syntax `local.file.example.content`, where `local.file.example` is the fully qualified name of the component (the component's type + its label) and `content` is the name of the export.

<p align="center">
<img src="/media/docs/agent/diagram-flow-by-example-basic-0.svg" alt="Flow of example pipeline with local.file and prometheus.remote_write components" width="200" />
</p>

{{< admonition type="note" >}}
The `local.file` component's label is set to `"example"`, so the fully qualified name of the component is `local.file.example`. The `prometheus.remote_write` component's label is set to `"local_prom"`, so the fully qualified name of the component is `prometheus.remote_write.local_prom`.
{{< /admonition >}}

This example pipeline still doesn't do anything, so let's add some more components to it.

## Shipping your first metrics

[prometheus.exporter.unix]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.exporter.unix/
[prometheus.scrape]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.scrape/
[prometheus.remote_write]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.remote_write/

**Recommended reading**

- Optional: [prometheus.exporter.unix][]
- Optional: [prometheus.scrape][]
- Optional: [prometheus.remote_write][]

Make a simple pipeline with a `prometheus.exporter.unix` component, a `prometheus.scrape` component to scrape it, and a `prometheus.remote_write` component to send the scraped metrics to Prometheus.

```river
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

Run {{< param "PRODUCT_NAME" >}} with:

```bash
/path/to/agent run config.river
```

Navigate to [http://localhost:3000/explore](http://localhost:3000/explore) in your browser. After ~15-20 seconds, you should be able to see the metrics from the `prometheus.exporter.unix` component! Try querying for `node_memory_Active_bytes` to see the active memory of your host.

<p align="center">
<img src="/media/docs/agent/screenshot-flow-by-example-memory-usage.png" alt="Screenshot of node_memory_Active_bytes query in Grafana" />
</p>

## Visualizing the relationship between components

The following diagram is an example pipeline:

<p align="center">
<img src="/media/docs/agent/diagram-flow-by-example-full-0.svg" alt="Flow of example pipeline with a prometheus.scrape, prometheus.exporter.unix, and prometheus.remote_write components" width="400" />
</p>

The preceding configuration defines three components:

- `prometheus.scrape` - A component that scrapes metrics from components that export targets.
- `prometheus.exporter.unix` - A component that exports metrics from the host, built around [node_exporter](https://github.com/prometheus/node_exporter).
- `prometheus.remote_write` - A component that sends metrics to a Prometheus remote-write compatible endpoint.

The `prometheus.scrape` component references the `prometheus.exporter.unix` component's targets export, which is a list of scrape targets. The `prometheus.scrape` component then forwards the scraped metrics to the `prometheus.remote_write` component.

One rule is that components can't form a cycle. This means that a component can't reference itself directly or indirectly. This is to prevent infinite loops from forming in the pipeline.

## Exercise for the reader

[prometheus.exporter.redis]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.exporter.redis/

**Recommended Reading**

- Optional: [prometheus.exporter.redis][]

Let's start a container running Redis and configure {{< param "PRODUCT_NAME" >}} to scrape metrics from it.

```bash
docker container run -d --name flow-redis -p 6379:6379 --rm redis
```

Try modifying the pipeline to scrape metrics from the Redis exporter. You can refer to the [prometheus.exporter.redis][] component documentation for more information on how to configure it.

To give a visual hint, you want to create a pipeline that looks like this:

<p align="center">
<img src="/media/docs/agent/diagram-flow-by-example-exercise-0.svg" alt="Flow of exercise pipeline, with a scrape, unix_exporter, redis_exporter, and remote_write component" width="600" />
</p>

{{< admonition type="note" >}}
[concat]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/stdlib/concat/

You may find the [concat][] standard library function useful.
{{< /admonition >}}

You can run {{< param "PRODUCT_NAME" >}} with the new configuration file by running:

```bash
/path/to/agent run config.river
```

Navigate to [http://localhost:3000/explore](http://localhost:3000/explore) in your browser. After the first scrape, you should be able to query for `redis` metrics as well as `node` metrics.

To shut down the Redis container, run:

```bash
docker container stop flow-redis
```

If you get stuck, you can always view a solution here:
{{< collapse title="Solution" >}}

```river
// Configure your first components, learn about the standard library, and learn how to run Grafana Agent

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

You might have noticed that running {{< param "PRODUCT_NAME" >}} with the configurations created a directory called `data-agent` in the directory you ran {{< param "PRODUCT_NAME" >}} from. This directory is where components can store data, such as the `prometheus.exporter.unix` component storing its WAL (Write Ahead Log). If you look in the directory, do you notice anything interesting? The directory for each component is the fully qualified name.

If you'd like to store the data elsewhere, you can specify a different directory by supplying the `--storage.path` flag to {{< param "PRODUCT_ROOT_NAME" >}}'s run command, for example, `/path/to/agent run config.river --storage.path /etc/grafana-agent`. Generally, you can use a persistent directory for this, as some components may use the data stored in this directory to perform their function.

In the next tutorial, you will look at how to configure {{< param "PRODUCT_NAME" >}} to collect logs from a file and send them to Loki. You will also look at using different components to process metrics and logs before sending them.
