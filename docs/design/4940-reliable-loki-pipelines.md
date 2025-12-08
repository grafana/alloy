# Proposal: Reliable Loki pipelines

* Author: Karl Persson (@kalleep)
* Last updated: 2025-11-30
* Discussion link: https://github.com/grafana/alloy/pull/4940

## Abstract

Alloy's Loki pipelines currently use channels, which limits throughput due to head-of-line blocking and can cause silent log drops during config reloads or shutdowns.

This proposal introduces a function-based pipeline using a `Consumer` or `Appender` interface, replacing the channel-based design.

Source components will call functions directly on downstream components, enabling parallel processing and returning errors that sources can use for retry logic or proper HTTP error responses.

## Problem

Loki pipelines in Alloy are built using (unbuffered) channels, a design inherited from promtail. 

This comes with two big limitations:
1. Throughput of each component is limited due to head-of-line blocking, where pushing to the next channel may not be possible in the presence of a slow component. An example of this is usage of [secretfilter](https://github.com/grafana/alloy/issues/3694).
2. Because there is no way to signal back to the source, components can silently drop logs during config reload or shutdown and there is no way to detect that.

Consider the following simplified config:
```
loki.source.file "logs" {
  targets    = targets
  forward_to = [loki.process.logs.receiver]
}

loki.process "logs" {
  forward_to = [loki.write.loki.receiver]
}

loki.write "loki" {}
```

`loki.source.file` will tail all files from targets and compete to send on the channel exposed by `loki.process`. Only one entry will be processed by each stage configured in `loki.process`. If a reload happens or if Alloy is shutting down, logs could be silently dropped.

There is also no way to abort entries in the pipeline. This is problematic when using components such as `loki.source.api` where caller could cancel request due to e.g. timeouts.

## Proposal 0: Do nothing
This architecture works in most cases, it will be hard to use slow components such as `secretfilter` because a lot of the time it's too slow.
It's also hard to use Alloy as a gateway for loki pipelines with e.g. `loki.source.api` due to the limitations listed above.

## Proposal 1: Chain function calls

Loki pipelines are the only ones using channels for passing data between components. Prometheus, Pyroscope and otelcol are all using this pattern where each component just calls functions on the next.

They all have slightly different interfaces but basically work the same. Each component exports its own interface like Appender for Prometheus or Consumer for Otel.

We could adopt the same pattern for loki pipelines as well with the following interface:

```go
type Consumer interface {
	Consume(ctx context.Context, entries []Entry) error
}
```

Adopting this pattern for loki pipelines would change it from a channel-based pipeline to a function-based pipeline. This would give us two things:
1. Increased throughput because several sources such as many files or http requests can now call the next component in the pipeline at the same time.
2. A way to return signals back to the source so we can handle things like giving a proper error response or determine if the position file should be updated. 

Solving the issues listed above.

A batch of entries should be considered successfully consumed when they are queued up for sending. We could try to extend this to when it was successfully sent over the wire, but that could be considered an improvement at a later stage.

Pros:
* Increase throughput of log pipelines.
* A way to signal back to the source
Cons:
* We need to rewrite all loki components with this new pattern and make them safe to call in parallel.
* We go from an iterator-like pipeline to passing slices around. Every component would have to iterate over this slice and we need to make sure it's safe to mutate because of fan-out.

## Proposal 2: Appendable

The prometheus pipeline uses [Appendable](https://github.com/prometheus/prometheus/blob/main/storage/interface.go#L62).
Appendable only has one method `Appender` that will return an implementation of [Appender](https://github.com/prometheus/prometheus/blob/main/storage/interface.go#L270).

We could adopt this pattern for loki pipelines by having:
```go
type Appendable interface {
    Appender(ctx context.Context) Appender
}

type Appender interface {
    Append(entry Entry) error
    Commit() error
    Rollback() error
}
```

This approach would, like Proposal 1, solve the issues listed above with a function-based pipeline, but the pipeline would still be iterator-like (one entry at a time).

### How it works
Source components would: 
Obtain an `Appender` that can fan-out to all downstream components, then call `Append` for each entry.
If every call to `Append` is successful, `Commit` should be called; otherwise `Rollback`.

Processing components would:
Implement `Appendable` to return an `Appender` that runs processing for each entry and fan-out to all downstream components.

Sink components would:
Implement `Appendable` to return an `Appender` that buffers entries until either `Commit` or `Rollback` is called.

Pros:
* Increase throughput of log pipelines.
* A way to signal back to the source
* Iterator-like pipeline - one entry at a time
* Transaction semantics, sources have better control on when a batch should be aborted.
Cons:
* We need to rewrite all loki components with this new pattern and make them safe to call in parallel.
* More complex API

## Considerations for implementation

### Handling fan-out failures

Because a pipeline can "fan-out" to multiple paths, it can also partially succeed. We need to determine how to handle this.

Two options to handle this:
* Always retry if one or more failed - This could lead to duplicated logs but is easy and safe to implement. This is also how otelcol works.
    * When using `loki.source.api`, we would return a 5xx error so the caller can retry.
    * When using `loki.source.file`, we would retry the same batch again.
* Configuration option `min_success` - Only retry if we don't succeed on at least the configured number of destinations.

### Transition from current pipeline to either Proposal 1 or Proposal 2
Changing the way loki pipeline works is a big effort and will affect all loki components.

We have a couple of options how to do this:
1. Build tag
    * We build out the new pipeline under a build tag. This way we could build custom Alloy image using this new pipeline and test it out internally before we commit it to an official release.
2. New argument
    * We could add additional argument to components in addition to `forward_to`. This new argument would be using the new pipeline code. This argument would be protected by experimental flag and we would remove it once we are confident in the new code and remove the current pipeline.
3. Replace pipeline directly
    * We could replace the pipeline directly without any fallback mechanism. This should be doable over several PRs where we first only replace the communication between components, e.g. in loki.source.file we would still have the [main loop](https://github.com/grafana/alloy/blob/main/internal/component/loki/source/file/file.go#L229-L247) reading from channel and send one entry at a time with this new pipeline between components. Then we could work component by component and remove most of channel usage.

### Affected components

The following components need to be updated with this new interface and we need to make sure they are concurrency safe:

**Source components** (need to call `Consume()` and handle errors):
- `loki.source.file`
- `loki.source.api`
- `loki.source.kafka`
- `loki.source.journal`
- `loki.source.docker`
- `loki.source.kubernetes`
- `loki.source.kubernetes_events`
- `loki.source.podlogs`
- `loki.source.syslog`
- `loki.source.gelf`
- `loki.source.cloudflare`
- `loki.source.gcplog`
- `loki.source.heroku`
- `loki.source.azure_event_hubs`
- `loki.source.aws_firehose`
- `loki.source.windowsevent`
- `database_observability.mysql`
- `database_observability.postgres`
- `faro.receiver`

**Processing components** (need to implement `Consumer` and forward to next):
- `loki.process`
- `loki.relabel`
- `loki.secretfilter`
- `loki.enrich`

**Sink components** (need to implement `Consumer`):
- `loki.write`
- `loki.echo`
