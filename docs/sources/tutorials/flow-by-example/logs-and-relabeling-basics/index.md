---
aliases:
- /docs/grafana-cloud/agent/flow/tutorials/flow-by-example/logs-and-relabeling-basics/
- /docs/grafana-cloud/monitor-infrastructure/agent/flow/tutorials/flow-by-example/logs-and-relabeling-basics/
- /docs/grafana-cloud/monitor-infrastructure/integrations/agent/flow/tutorials/flow-by-example/logs-and-relabeling-basics/
- /docs/grafana-cloud/send-data/agent/flow/tutorials/logs-and-relabeling-basics/
canonical: https://grafana.com/docs/agent/latest/flow/tutorials/flow-by-example/logs-and-relabeling-basics/
description: Learn how to relabel metrics and collect logs
title: Logs and relabeling basics
weight: 30
---

# Logs and relabeling basics

This tutorial assumes you have completed the [First components and introducing the standard library](https://grafana.com/docs/agent/<AGENT_VERSION>/flow/tutorials/flow-by-example/first-components-and-stdlib/) tutorial, or are at least familiar with the concepts of components, attributes, and expressions and how to use them. You will cover some basic metric relabeling, followed by how to send logs to Loki.

## Relabel metrics

[prometheus.relabel]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.relabel/

**Recommended reading**

- Optional: [prometheus.relabel][]

Before moving on to logs, let's look at how we can use the `prometheus.relabel` component to relabel metrics. The `prometheus.relabel` component allows you to perform Prometheus relabeling on metrics and is similar to the `relabel_configs` section of a Prometheus scrape config.

Let's add a `prometheus.relabel` component to a basic pipeline and see how to add labels.

```river
prometheus.exporter.unix "localhost" { }

prometheus.scrape "default" {
    scrape_interval = "10s"

    targets    = prometheus.exporter.unix.localhost.targets
    forward_to = [
        prometheus.relabel.example.receiver,
    ]
}

prometheus.relabel "example" {
    forward_to = [
        prometheus.remote_write.local_prom.receiver,
    ]

    rule {
        action       = "replace"
        target_label = "os"
        replacement  = constants.os
    }
}

prometheus.remote_write "local_prom" {
    endpoint {
        url = "http://localhost:9090/api/v1/write"
    }
}
```

We have now created the following pipeline:

![Diagram of pipeline that scrapes prometheus.exporter.unix, relabels the metrics, and remote_writes them](/media/docs/agent/diagram-flow-by-example-relabel-0.svg)

This pipeline has a `prometheus.relabel` component that has a single rule.
This rule has the `replace` action, which will replace the value of the `os` label with a special value: `constants.os`.
This value is a special constant that is replaced with the OS of the host {{< param "PRODUCT_ROOT_NAME" >}} is running on.
You can see the other available constants in the [constants](https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/stdlib/constants/) documentation.
This example has one rule block, but you can have as many as you want.
Each rule block is applied in order.

If you run {{< param "PRODUCT_ROOT_NAME" >}} and navigate to [localhost:3000/explore](http://localhost:3000/explore), you can see the `os` label on the metrics. Try querying for `node_context_switches_total` and look at the labels.

Relabeling uses the same rules as Prometheus. You can always refer to the [prometheus.relabel documentation](https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.relabel/#rule-block) for a full list of available options.

{{< admonition type="note" >}}
You can forward multiple components to one `prometheus.relabel` component. This allows you to apply the same relabeling rules to multiple pipelines.
{{< /admonition >}}

{{< admonition type="warning" >}}
There is an issue commonly faced when relabeling and using labels that start with `__` (double underscore). These labels are considered internal and are dropped before relabeling rules from a `prometheus.relabel` component are applied. If you would like to keep or act on these kinds of labels, use a [discovery.relabel](https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/discovery.relabel/) component.
{{< /admonition >}}

## Send logs to Loki

[local.file_match]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/local.file_match/
[loki.source.file]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/loki.source.file/
[loki.write]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/loki.write/

**Recommended reading**

- Optional: [local.file_match][]
- Optional: [loki.source.file][]
- Optional: [loki.write][]

Now that you're comfortable creating components and chaining them together, let's collect some logs and send them to Loki. We will use the `local.file_match` component to perform file discovery, the `loki.source.file` to collect the logs, and the `loki.write` component to send the logs to Loki.

Before doing this, we need to ensure we have a log file to scrape. We will use the `echo` command to create a file with some log content.

```bash
mkdir -p /tmp/flow-logs
echo "This is a log line" > /tmp/flow-logs/log.log
```

Now that we have a log file, let's create a pipeline to scrape it.

```river
local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/flow-logs/*.log"}]
}

loki.source.file "local_files" {
    targets    = local.file_match.tmplogs.targets
    forward_to = [loki.write.local_loki.receiver]
}

loki.write "local_loki" {
    endpoint {
        url = "http://localhost:3100/loki/api/v1/push"
    }
}
```

The rough flow of this pipeline is:

![Diagram of pipeline that collects logs from /tmp/flow-logs and writes them to a local Loki instance](/media/docs/agent/diagram-flow-by-example-logs-0.svg)

If you navigate to [localhost:3000/explore](http://localhost:3000/explore) and switch the Datasource to `Loki`, you can query for `{filename="/tmp/flow-logs/log.log"}` and see the log line we created earlier. Try running the following command to add more logs to the file.

```bash
echo "This is another log line!" >> /tmp/flow-logs/log.log
```

If you re-execute the query, you can see the new log lines.

![Grafana Explore view of example log lines](/media/docs/agent/screenshot-flow-by-example-log-lines.png)

If you are curious how {{< param "PRODUCT_ROOT_NAME" >}} keeps track of where it is in a log file, you can look at `data-agent/loki.source.file.local_files/positions.yml`. 
If you delete this file, {{< param "PRODUCT_ROOT_NAME" >}} starts reading from the beginning of the file again, which is why keeping the {{< param "PRODUCT_ROOT_NAME" >}}'s data directory in a persistent location is desirable.

## Exercise

[loki.relabel]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/loki.relabel/
[loki.process]: https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/loki.process/

**Recommended reading**

- [loki.relabel][]
- [loki.process][]

### Add a Label to Logs

This exercise will have two parts, building on the previous example. Let's start by adding an `os` label (just like the Prometheus example) to all of the logs we collect.

Modify the following snippet to add the label `os` with the value of the `os` constant.

```river
local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/flow-logs/*.log"}]
}

loki.source.file "local_files" {
    targets    = local.file_match.tmplogs.targets
    forward_to = [loki.write.local_loki.receiver]
}

loki.write "local_loki" {
    endpoint {
        url = "http://localhost:3100/loki/api/v1/push"
    }
}
```

{{< admonition type="note" >}}
You can use the [loki.relabel](https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/loki.relabel) component to relabel and add labels, just like you can with the [prometheus.relabel](https://grafana.com/docs/agent/<AGENT_VERSION>/flow/reference/components/prometheus.relabel) component.
{{< /admonition >}}

Once you have your completed configuration, run {{< param "PRODUCT_ROOT_NAME" >}} and execute the following:

```bash
echo 'level=info msg="INFO: This is an info level log!"' >> /tmp/flow-logs/log.log
echo 'level=warn msg="WARN: This is a warn level log!"' >> /tmp/flow-logs/log.log
echo 'level=debug msg="DEBUG: This is a debug level log!"' >> /tmp/flow-logs/log.log
```

Navigate to [localhost:3000/explore](http://localhost:3000/explore) and switch the Datasource to `Loki`. Try querying for `{filename="/tmp/flow-logs/log.log"}` and see if you can find the new label!

Now that we have added new labels, we can also filter on them. Try querying for `{os!=""}`. You should only see the lines you added in the previous step.

{{< collapse title="Solution" >}}

```river
// Let's learn about relabeling and send logs to Loki!

local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/flow-logs/*.log"}]
}

loki.source.file "local_files" {
    targets    = local.file_match.tmplogs.targets
    forward_to = [loki.relabel.add_static_label.receiver]
}

loki.relabel "add_static_label" {
    forward_to = [loki.write.local_loki.receiver]

    rule {
        target_label = "os"
        replacement  = constants.os
    }
}

loki.write "local_loki" {
    endpoint {
        url = "http://localhost:3100/loki/api/v1/push"
    }
}
```

{{< /collapse >}}

### Extract and add a Label from Logs

{{< admonition type="note" >}}
This exercise is more challenging than the previous one. If you are having trouble, skip it and move to the next section, which will cover some of the concepts used here. You can always come back to this exercise later.
{{< /admonition >}}

This exercise will build on the previous one, though it's more involved. 

Let's say we want to extract the `level` from the logs and add it as a label. As a starting point, look at [loki.process][].
This component allows you to perform processing on logs, including extracting values from log contents.

Try modifying your configuration from the previous section to extract the `level` from the logs and add it as a label.
If needed, you can find a solution to the previous exercise at the end of the [previous section](#add-a-label-to-logs).

{{< admonition type="note" >}}
The `stage.logfmt` and `stage.labels` blocks for `loki.process` may be helpful.
{{< /admonition >}}

Once you have your completed config, run {{< param "PRODUCT_ROOT_NAME" >}} and execute the following:

```bash
echo 'level=info msg="INFO: This is an info level log!"' >> /tmp/flow-logs/log.log
echo 'level=warn msg="WARN: This is a warn level log!"' >> /tmp/flow-logs/log.log
echo 'level=debug msg="DEBUG: This is a debug level log!"' >> /tmp/flow-logs/log.log
```

Navigate to [localhost:3000/explore](http://localhost:3000/explore) and switch the Datasource to `Loki`. Try querying for `{level!=""}` to see the new labels in action.

![Grafana Explore view of example log lines, now with the extracted 'level' label](/media/docs/agent/screenshot-flow-by-example-log-line-levels.png)

{{< collapse title="Solution" >}}

```river
// Let's learn about relabeling and send logs to Loki!

local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/flow-logs/*.log"}]
}

loki.source.file "local_files" {
    targets    = local.file_match.tmplogs.targets
    forward_to = [loki.process.add_new_label.receiver]
}

loki.process "add_new_label" {
    // Extract the value of "level" from the log line and add it to the extracted map as "extracted_level"
    // You could also use "level" = "", which would extract the value of "level" and add it to the extracted map as "level"
    // but to make it explicit for this example, we will use a different name.
    //
    // The extracted map will be covered in more detail in the next section.
    stage.logfmt {
        mapping = {
            "extracted_level" = "level",
        }
    }

    // Add the value of "extracted_level" from the extracted map as a "level" label
    stage.labels {
        values = {
            "level" = "extracted_level",
        }
    }

    forward_to = [loki.relabel.add_static_label.receiver]
}

loki.relabel "add_static_label" {
    forward_to = [loki.write.local_loki.receiver]

    rule {
        target_label = "os"
        replacement  = constants.os
    }
}

loki.write "local_loki" {
    endpoint {
        url = "http://localhost:3100/loki/api/v1/push"
    }
}
```

{{< /collapse >}}

## Finishing up and next steps

You have learned the concepts of components, attributes, and expressions. You have also seen how to use some standard library components to collect metrics and logs. In the next tutorial, you will learn more about how to use the `loki.process` component to extract values from logs and use them.

