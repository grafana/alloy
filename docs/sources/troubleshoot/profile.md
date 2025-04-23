---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/profile/
description: Learn how to profile resource consumption
title: Profile Grafana Alloy resource consumption
menuTitle: Profile resource consumption
weight: 300
---

# Profile {{% param "FULL_PRODUCT_NAME" %}} resource consumption

{{< param "PRODUCT_NAME" >}} is written in the Go programming language, which offers [built-in][pprof-pkg] support for profiling.
Like other applications written in Go, you can profile {{< param "PRODUCT_NAME" >}} by sending an HTTP request, which returns a pprof Go profile.

After you have the pprof file, visualize it as a [flame graph][flame-graph] in [Grafana Pyroscope][pyroscope-getstarted].
Alternatively, you could visualize it on [Grafana Play][pyroscope-adhoc] or locally by using Go's built-in [pprof tool][go-pprof].

{{< admonition type="note" >}}
A profile may contain sensitive information about your environment.
You may not want to upload your profiles to a public location.
{{< /admonition >}}

The port you use to send the HTTP request is controlled by the `--server.http.listen-addr` [command line argument][cmd-cli].
It's set to `127.0.0.1:12345` by default.

[pprof-pkg]: https://pkg.go.dev/net/http/pprof/
[pyroscope-adhoc]: https://play.grafana.org/a/grafana-pyroscope-app/ad-hoc
[go-pprof]: https://go.dev/blog/pprof/
[pyroscope-getstarted]: https://grafana.com/docs/pyroscope/latest/get-started/
[flame-graph]: https://grafana.com/docs/pyroscope/latest/view-and-analyze-profile-data/flamegraphs/
[cmd-cli]: ../../reference/cli/run/

## Obtain a single profile

Different types of HTTP requests retrieve different profiles.

### Memory consumption

Goroutine leaks often cause memory leaks.
This is why obtaining a goroutine profile is usually necessary when investigating memory issues.
For example:

```bash
curl localhost:12345/debug/pprof/heap -o heap.pprof
curl localhost:12345/debug/pprof/goroutine -o goroutine.pprof
```

It's often helpful to collect profiles both when memory usage is low and when it's high.
You can compare the profiles, and it may be easier to identify what caused the memory consumption to increase.

### CPU consumption

If you are experiencing high CPU consumption, you can collect a CPU profile:

```bash
curl http://localhost:12345/debug/pprof/profile?seconds=30 -o cpu.pprof
```

The `?seconds=30` part of the URL above means the profiling continues for 30 seconds.

## Continuous profiling

You don't have to send manual `curl` commands each time you want to collect profiles.
You can also profile continuously using the [pyroscope components][] in {{< param "PRODUCT_NAME" >}}.

If you have very few {{< param "PRODUCT_NAME" >}} instances, you can even configure them to profile themselves.
However, if you have a large cluster of collectors, it's best to set up {{< param "PRODUCT_NAME" >}} instances whose sole job is to profile other {{< param "PRODUCT_NAME" >}} instances.

The following is an example of an {{< param "PRODUCT_NAME" >}} instance profiling itself:

```alloy
pyroscope.scrape "default" {
  targets    = [{"__address__" = "localhost:12345", "service_name"="alloy"}]
  forward_to = [pyroscope.write.default.receiver]
}

pyroscope.write "default" {
  endpoint {
    url = "https://profiles-prod-014.grafana.net"
    basic_auth {
      username = sys.env("PYROSCOPE_USERNAME")
      password = sys.env("PYROSCOPE_PASSWORD")
    }
  }
}
```

[pyroscope components]: ../../reference/components/pyroscope/

## Expected resource consumption

Refer to [Estimate resource usage][res-usage] for more information about the expected resource consumption in {{< param "PRODUCT_NAME" >}}.

[res-usage]: ../../introduction/estimate-resource-usage/

## {{% param "PRODUCT_NAME" %}} consumes an abnormally large amount of resources

If {{< param "PRODUCT_NAME" >}} consumes an abnormally large amount of resources, you can open an issue in the [Alloy repository][alloy-repo].
Attach your pprof files and your {{< param "PRODUCT_NAME" >}} configuration file.
Make sure you redact any secrets in the attachments.

[alloy-repo]: https://github.com/grafana/alloy/issues/
