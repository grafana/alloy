---
canonical: https://grafana.com/docs/alloy/latest/troubleshoot/profile/
description: Learn how to profile resource consumption
title: Profile Grafana Alloy resource consumption
menuTitle: Profile resource consumption
weight: 300
---

# Profile Grafana Alloy resource consumption

Alloy is written in the Go programming language, which offers [built-in][pprof-pkg] support for profiling.
Just like other applications written in Go, you can profile Alloy by sending an HTTP request which returns you a "pprof" Go profile.

Once you have the pprof file, simply visualize it as a [flame graph][flame-graph] in [Grafana Pyroscope][pyroscope-getstarted].
Alternatively, you could visualize it on [Grafana Play][pyroscope-adhoc] or by using Go's built-in [pprof tool][go-pprof] locally.

{{< admonition type="note" >}}
A profile may contain sensitive information about your environment.
You may not want to upload your profiles to a public location.
{{< /admonition >}}

The port on which you need to send the HTTP request is governed by Alloy's `--server.http.listen-addr` [command line argument][cmd-cli].
It is set to `127.0.0.1:12345` by default.

[pprof-pkg]: https://pkg.go.dev/net/http/pprof
[pyroscope-adhoc]: https://play.grafana.org/a/grafana-pyroscope-app/ad-hoc
[go-pprof]: https://go.dev/blog/pprof
[pyroscope-getstarted]: https://grafana.com/docs/pyroscope/latest/get-started/
[flame-graph]: https://grafana.com/docs/pyroscope/latest/view-and-analyze-profile-data/flamegraphs/
[cmd-cli]: ../../reference/cli/run

## Obtaining a single profile

Different types of HTTP requests will retrieve different profiles.

### Memory consumption

Memory leaks are often caused by goroutine leaks. 
This is why obtaining a goroutine profile is usually necessary when investigating memory issues.
For example:

```
curl localhost:12345/debug/pprof/heap -o heap.pprof
curl localhost:12345/debug/pprof/goroutine -o goroutine.pprof
```

It is often helpful to collect profiles both when the memory usage is low, and also when it is high.
That way it may be more clear what caused the consumption to increase.

### CPU consumption

If you are experiencing high CPU consumption, you can collect a CPU profile:

```
curl http://localhost:12345/debug/pprof/profile?seconds=30 -o cpu.pprof
```

The `?seconds=30` part of the URL above means that the profiling will continue for 30 seconds.

## Continuous profiling

You don't have to send manual `curl` commands every time you want to collect profiles.
You can also profile continuously using Alloy's built-in [pyroscope components][].

If you have very few Alloy instances, you can even configure them to profile themselves.
However, if you have a large cluster of collectors, it is best to set up Alloy instances whose sole job is to profile other Alloy instances.

An example of an Alloy instance profiling itself:

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

[pyroscope components]: ../../reference/components/pyroscope

## What is Alloy's expected resource consumption?

Refer to the [Estimate resource usage][res-usage] page.

[res-usage]: ../../introduction/estimate-resource-usage

## What if my Alloy instance consumes an abnormally large amount of resources?

You could open an issue in [Alloy's repository][alloy-repo]. 
It would be helpful to attach pprof files and Alloy's configuration file.
Make sure any secrets are redacted.

[alloy-repo]: https://github.com/grafana/alloy/issues