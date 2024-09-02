---
canonical: https://grafana.com/docs/alloy/latest/tutorials/logs-and-relabeling-basics/
description: Learn how to relabel metrics and collect logs
menuTitle: Logs and relabeling basics
title: Logs and relabeling basics in Grafana Alloy
weight: 250
---

# Logs and relabeling basics in {{% param "FULL_PRODUCT_NAME" %}}

This tutorial covers some basic metric relabeling, and shows you how to send logs to Loki.

## Before you begin

To complete this tutorial:

* You must complete the [First components and the standard library][first] tutorial.

## Relabel metrics

Now that you have built a basic pipeline and scraped some metrics, you can use the `prometheus.relabel` component to relabel metrics.

### Recommended reading

- Optional: [prometheus.relabel][]

### Add a `prometheus.relabel` component to your pipeline

The `prometheus.relabel` component allows you to perform Prometheus relabeling on metrics and is similar to the `relabel_configs` section of a Prometheus scrape configuration.

Add a `prometheus.relabel` component to a basic pipeline and add labels.

```alloy
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

You have created the following pipeline:

{{< figure src="/media/docs/alloy/diagram-example-relabel-alloy.png" alt="Diagram of pipeline that scrapes prometheus.exporter.unix, relabels the metrics, and remote_writes them" >}}

This pipeline has a `prometheus.relabel` component that has a single rule.
This rule has the `replace` action, which will replace the value of the `os` label with a special value: `constants.os`.
This value is a special constant that is replaced with the OS of the host {{< param "PRODUCT_NAME" >}} is running on.
You can see the other available constants in the [constants][] documentation.
This example has one rule block, but you can have as many as you want.
Each rule block is applied in order.

If you run {{< param "PRODUCT_NAME" >}} and navigate to [localhost:3000/explore][], you can see the `os` label on the metrics.
Try querying for `node_context_switches_total` and look at the labels.

Relabeling uses the same rules as Prometheus. You can always refer to the [prometheus.relabel rule-block][] documentation for a full list of available options.

{{< admonition type="note" >}}
You can forward multiple components to one `prometheus.relabel` component. This allows you to apply the same relabeling rules to multiple pipelines.
{{< /admonition >}}

{{< admonition type="warning" >}}
There is an issue commonly faced when relabeling and using labels that start with `__` (double underscore).
These labels are considered internal and are dropped before relabeling rules from a `prometheus.relabel` component are applied.
If you would like to keep or act on these kinds of labels, use a [discovery.relabel][] component.

[discovery.relabel]: ../../reference/components/discovery/discovery.relabel/
{{< /admonition >}}

## Send logs to Loki

Now that you've created components and chained them together, you can collect some logs and send them to Loki.

### Recommended reading

- Optional: [local.file_match][]
- Optional: [loki.source.file][]
- Optional: [loki.write][]

### Find and collect the logs

You can use the `local.file_match` component to perform file discovery, the `loki.source.file` to collect the logs, and the `loki.write` component to send the logs to Loki.

Before doing this, make sure you have a log file to scrape.
You can use the `echo` command to create a file with some log content.

```bash
mkdir -p /tmp/alloy-logs
echo "This is a log line" > /tmp/alloy-logs/log.log
```

Now that you have a log file, you can create a pipeline to scrape it.

```alloy
local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/alloy-logs/*.log"}]
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

{{< figure src="/media/docs/alloy/diagram-example-logs-loki-alloy.png" width="500" alt="Diagram of pipeline that collects logs from /tmp/alloy-logs and writes them to a local Loki instance" >}}

If you navigate to [localhost:3000/explore][] and switch the Datasource to `Loki`, you can query for `{filename="/tmp/alloy-logs/log.log"}` and see the log line we created earlier.
Try running the following command to add more logs to the file.

```bash
echo "This is another log line!" >> /tmp/alloy-logs/log.log
```

If you re-execute the query, you can see the new log lines.

{{< figure src="/media/docs/alloy/screenshot-log-lines.png" alt="Grafana Explore view of example log lines" >}}

If you are curious how {{< param "PRODUCT_NAME" >}} keeps track of where it's in a log file, you can look at `data-alloy/loki.source.file.local_files/positions.yml`.
If you delete this file, {{< param "PRODUCT_NAME" >}} starts reading from the beginning of the file again, which is why keeping the {{< param "PRODUCT_NAME" >}}'s data directory in a persistent location is desirable.

## Exercise

The following exercise guides you through adding a label to the logs, and filtering the results.

### Recommended reading

- [loki.relabel][]
- [loki.process][]

### Add a Label to Logs

This exercise has two parts, and builds on the previous example.
Start by adding an `os` label (just like the Prometheus example) to all of the logs we collect.

Modify the following snippet to add the label `os` with the value of the `os` constant.

```alloy
local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/alloy-logs/*.log"}]
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

{{< admonition type="tip" >}}
You can use the [loki.relabel][] component to relabel and add labels, just like you can with the [prometheus.relabel][] component.

[loki.relabel]: ../../reference/components/loki/loki.relabel
[prometheus.relabel]: ../../reference/components/prometheus/prometheus.relabel
{{< /admonition >}}

Run {{< param "PRODUCT_NAME" >}} and execute the following:

```bash
echo 'level=info msg="INFO: This is an info level log!"' >> /tmp/alloy-logs/log.log
echo 'level=warn msg="WARN: This is a warn level log!"' >> /tmp/alloy-logs/log.log
echo 'level=debug msg="DEBUG: This is a debug level log!"' >> /tmp/alloy-logs/log.log
```

Navigate to [localhost:3000/explore][] and switch the Datasource to `Loki`.
Try querying for `{filename="/tmp/alloy-logs/log.log"}` and see if you can find the new label.

Now that you have added new labels, you can also filter on them. Try querying for `{os!=""}`.
You should only see the lines you added in the previous step.

{{< collapse title="Solution" >}}

```alloy
// Let's learn about relabeling and send logs to Loki!

local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/alloy-logs/*.log"}]
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
This exercise is more challenging than the previous one.
If you are having trouble, skip it and move to the next section, which covers some of the concepts used here.
You can always come back to this exercise later.
{{< /admonition >}}

This exercise builds on the previous one, though it's more involved.

Assume you want to extract the `level` from the logs and add it as a label. As a starting point, look at [loki.process][].
This component allows you to perform processing on logs, including extracting values from log contents.

Try modifying your configuration from the previous section to extract the `level` from the logs and add it as a label.
If needed, you can find a solution to the previous exercise at the end of the [previous section](#add-a-label-to-logs).

{{< admonition type="tip" >}}
The `stage.logfmt` and `stage.labels` blocks for `loki.process` may be helpful.
{{< /admonition >}}

Run {{< param "PRODUCT_NAME" >}} and execute the following:

```bash
echo 'level=info msg="INFO: This is an info level log!"' >> /tmp/alloy-logs/log.log
echo 'level=warn msg="WARN: This is a warn level log!"' >> /tmp/alloy-logs/log.log
echo 'level=debug msg="DEBUG: This is a debug level log!"' >> /tmp/alloy-logs/log.log
```

Navigate to [http://localhost:3000/explore](http://localhost:3000/explore) and switch the Datasource to `Loki`.
Try querying for `{level!=""}` to see the new labels in action.

{{< figure src="/media/docs/alloy/screenshot-log-line-levels.png" alt="Grafana Explore view of example log lines, now with the extracted 'level' label" >}}

{{< collapse title="Solution" >}}

```alloy
// Let's learn about relabeling and send logs to Loki!

local.file_match "tmplogs" {
    path_targets = [{"__path__" = "/tmp/alloy-logs/*.log"}]
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

You have learned the concepts of components, attributes, and expressions.
You have also seen how to use some standard library components to collect metrics and logs.
In the next tutorial, you learn more about how to use the `loki.process` component to extract values from logs and use them.

[first]: ../first-components-and-stdlib/
[prometheus.relabel]: ../../reference/components/prometheus/prometheus.relabel/
[constants]: ../../reference/stdlib/constants/
[prometheus.relabel rule-block]: ../../reference/components/prometheus/prometheus.relabel/#rule-block
[local.file_match]: ../../reference/components/local/local.file_match/
[loki.source.file]: ../../reference/components/loki/loki.source.file/
[loki.write]: ../../reference/components/loki/loki.write/
[loki.relabel]: ../../reference/components/loki/loki.relabel/
[loki.process]: ../../reference/components/loki/loki.process/
