# Proposal: Reliable Loki pipelines

- Author: Karl Persson (@kalleep), Piotr Gwizdala (@thampiotr)
- Last updated: 2026-10-26
- Discussion link: https://github.com/grafana/alloy/pull/4940

## Abstract

Alloy's Loki pipelines have correctness, error handling, and performance issues. We cover them together as they are related, and some solutions will address multiple problems. After an overview, we list our assumptions and propose solutions.

## Problems

Loki pipelines in Alloy are built using unbuffered Go channels, a design inherited from promtail.

This implementation comes with limitations that we aim to solve:

1. **Propagating errors:** There is no way for a downstream component to signal error or success. If we do receive errors in source components, we wouldn’t know how to handle them - the correct handling needs to be designed:
   - In some cases, an error can be propagated, for example in case of loki.source.api, we can respond to a client with a retriable error.
   - In other cases, an error may need to be retried, for example in case of loki.source.file.
   - Success handling: ideally we should only advance the positions in the file after we got success. Similarly, we should send response ‘200’ only when successful.
   - Logs can be dropped in the pipeline: the dropped logs should be considered successfully processed
   - Logs can be joined from multiple entries into one entry via multiline: in this case the success or failure of the aggregated line needs to be propagated as success/failure of all the entries that it was made with.

2. **Fan-out errors:** Properly handle errors when fan-out to 2+ subpipelines does not succeed. We need to define the desired behavior and what the ‘success’ means in the context of fan-out. See additional questions below.

3. **Loss of logs during shutdown:** because the component shutdown sequence in Alloy is currently not deterministic. To cleanly shutdown and drain Loki pipelines without losing logs, we want to stop accepting new logs and then make sure the pipeline is drained.

4. **Performance:** Throughput does not scale with CPU/memory given to the process. Pipelines don’t scale because of head-of-line blocking, where pushing to the next channel may not be possible in the presence of a slow component. An example of this is usage of [secretfilter](https://github.com/grafana/alloy/issues/3694).

5. **Traffic shedding:** When traffic volume is higher than Alloy’s max throughput, there is no mechanism to shed traffic and reject new requests without attempting to process them.
   Ideally we would shed traffic immediately on arrival if Alloy detects congestion, so that we can allow requests already in the pipeline to be processed. Perhaps it’s a temporary spike or scaling needs to kick-in.

6. **Congestion observability:** There is no way to track pipeline latency, for example, to understand if Alloy is able to keep up with the volume of logs. There is this GH issue.

7. **Cancelling writes:** There is no way to signal from the source to downstream components that processing of some entries is no longer needed (e.g. request cancelled). For example:
   loki.source.api receives a request which is subsequently cancelled by the client. If the logs are already sent, there’s nothing we can do about it, but if the logs are still in the pipeline, perhaps we could cancel their processing.

## Assumptions

**Error Handling for Fan-Out to Multiple Subpipelines**

When fanning out to two or more subpipelines, we considered several approaches for handling errors:

1. Partial success model – Treat the operation as successful if at least one downstream component succeeds.

2. All-or-nothing model – Require all downstream components to succeed for the overall operation to succeed.

3. Per-source configurable threshold – Allow each source component to define a minimum success, for example min_success, that determines how many downstream components must succeed.

4. Configurable per downstream edge – Allow downstream edge components to define whether their failures should impact the overall result, with configuration possible either at the edge component level or per endpoint, for example within loki.write.

**Our default behavior will follow the all-or-nothing model.** The overall operation succeeds only if all downstream components succeed. This provides clear and predictably safe semantics by default. Downstream components retry their own failures, so in practice a fan-out only returns once every downstream component has accepted the batch, and a slow downstream component holds up the others.

**Option 4** can be supported with the already existing `block_on_overflow` in `loki.write`, which defaults to `true`. With `block_on_overflow = false`, an endpoint whose queue is full drops the logs instead of blocking. The drop is counted in metrics and not reported as an error, so that endpoint cannot bottleneck the pipeline.

**Success Semantics for loki.write**

For loki.write we need to define when a batch of logs are considered successfully processed and when a success result can be returned and propagated to the source. We considered the following options:

1. When sent over the wire or written to disk
   - If WAL is disabled, consider the write successful only after the data has been successfully sent over the network.
   - If WAL is enabled, consider the write successful once the data is persisted to disk, since it is durable and survives crashes.

2. When written to the queue or written to disk.
   - If WAL is disabled, consider write to be successful once it is added to the in-memory queue. With a clean shutdown and no downstream issues, no logs are lost.
   - If WAL is enabled, similar to option 1, consider the write successful once the data is persisted to disk. This will be documented as the recommended configuration for users that want the additional durability.

**We have decided to adopt option 2.** While this approach is not as reliable as option 1 when using in-memory queue, the WAL option is equally reliable and would be recommended for users that want extra reliability. We can still revisit this later if we find that stronger guarantees are required.

## Goals

| Problem                                         | Impact                                                                                                                                                                                                                                                                                                                                                                                                                      | Priority     |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| Traffic shedding & Congestion observability     | There is always a performance limit. When we hit it, it should be well handled to give users a good experience and clarity of what’s going on.                                                                                                                                                                                                                                                                              | Very high    |
| Performance                                     | Crashing, not keeping up, loss of logs                                                                                                                                                                                                                                                                                                                                                                                      | Very high    |
| Loss of logs during shutdown                    | HPA can be quite active, we may end up losing logs. Some customers have alerts on logs.                                                                                                                                                                                                                                                                                                                                     | High         |
| Propagation of errors & fan-out errors handling | **So what errors do we need to handle?**<br>Seems like the only errors we could get here are: 1. I/O errors when writing to WAL. 2. Congestion when queues are full.<br><br>**For gateway setup**, requests that can be retried due to these errors may not be retried and we can lose logs.<br><br>**For position files** we only want to advance the position when the entries are successfully pushed into the pipeline. | Medium / Low |
| Cancelling writes                               | Potential for more duplicated logs, but duplicated logs are always possible.                                                                                                                                                                                                                                                                                                                                                | Medium / Low |

## Proposal: Function-based pipeline with synchronous error propagation

We replace the current unbuffered channel-based Loki pipeline with synchronous function calls. Components implement a shared Consumer interface. Source components wait for the full pipeline to complete before committing (responding to API clients or advancing file positions). Errors and context cancellation propagate back to the source naturally.

### Rollout

We refer to this proposal as Loki pipeline v2. It is implemented behind the experimental `--feature.loki.pipeline-v2.enabled` flag, which requires `--stability.level=experimental`. Each component keeps its current channel-based path unchanged and gets a separate path for pipeline v2. The flag applies to all Loki components at once: mixing components on the two pipelines is not supported.

### Interface

This is for illustration purposes only. The actual naming may be different.

```go
type Consumer interface {
  Consume(ctx context.Context, batch loki.Batch) error
}
```

This replaces the current Chan() chan<- loki.Entry. The call is synchronous: the caller's goroutine does the work through the pipeline. context.Context scopes the processing lifetime (HTTP request context, shutdown signal).

Consume() returns `nil`, a context error or `ErrConsumerStopped`. Components retry their own failures and only stop when the context is cancelled or the component stops. `ErrConsumerStopped` means a component in the pipeline has stopped, usually because a config reload removed it, and the source retries the batch. Any other error is unexpected and most likely a bug. When Consume() returns an error, the caller checks its own context with `ctx.Err() != nil` to tell a context error apart, instead of inspecting the returned error, since a component can return the error it was retrying when the context is cancelled.

Pipeline components like loki.process, loki.relabel, loki.write, and fan-out all implement Consumer. These components are stream-agnostic: they process whatever entries they receive. The entries they receive will already be grouped by stream.

### FanoutV2

Every component that can fan-out should use `FanoutV2`. This one is responsible to call all `Consume` on all consumers a component should forward to. Sources use it to forward to the pipeline.

```go
type FanoutV2 struct { ... }

// Consume calls consume on all internal consumers pass to it and aggregate errors.
func (f *FanoutV2) Consume(ctx context.Context, batch loki.Batch) error
```

`FanoutV2` takes a snapshot of its consumers for each call and delivers the batch to all of them. It returns the errors from its consumers together, including `ErrConsumerStopped`.

### Architecture

```
loki.source.api                            loki.source.file
     |                                          |
     | HTTP handler receives request            | Each tailer reads entries
     |                                          | (one origin per file)
     |                                          |
     | Consume() on the HTTP handler goroutine  | Consume() on the tailer goroutine
     | synchronous, may block                   | synchronous, may block
     v                                          v
+------------------------------------------------------------+
|                          FanoutV2                          |
+------------------------------------------------------------+
                          |
                          | Consume() - synchronous, may block
                          v
               +-- loki.process etc. --+
               |  (mutations,          |
               |   filtering)          |
               +-----------------------+
                          |
                          | Consume() - synchronous, may block
                          v
+------------------------------------------------------------+
|                      loki.write                            |
|                                                            |
|  Consume() -> append to in-memory queue or WAL             |
|  retries WAL errors until ctx is canceled or it stops      |
|  blocks if in-memory queue is full for backpressure        |
+------------------------------------------------------------+
```

The goroutine that calls Consume() handles the full processing path inline: the call passes through loki.process (mutations, filtering) and into loki.write. No goroutine hand-offs. Concurrency comes from the sources themselves, each HTTP request and each file tailer runs on its own goroutine.

loki.source.api. Calls Consume(ctx, batch) with all entries from the HTTP request. If success → respond 200. If the client disconnects, the HTTP request context is cancelled and processing aborts. No 200 is sent, so no commitment is made. If the context is cancelled because Alloy or the component is shutting down, or Consume() returns `ErrConsumerStopped` → respond with a retryable status code (503), so the client retries, possibly to another instance. Any other error from the pipeline is unexpected and most likely a bug → log it and respond 500. Apart from that, errors returned to the client only come from handling the request itself, such as parsing errors. To bound goroutine creation, loki.source.api will limit the number of concurrently accepted connections at the HTTP server level.

loki.source.file. Each tailer calls Consume(ctx, batch) for each batch read from its file (already single-stream). If success → advance position. If Consume() returns an error and the context is cancelled (shutdown or target removed) → do not advance, clean exit. If Consume() returns `ErrConsumerStopped` → do not advance, back off and send the batch again until it succeeds or the context is cancelled. The tailer keeps the lines it read until the batch is delivered and rebuilds the batch from them, since downstream components may have modified it. Any other error is unexpected and most likely a bug → log it and advance position.

loki.write. Implements Consumer by appending to a WAL or in-memory queue. Blocks if the queue is full and `block_on_overflow = true` (default). With `block_on_overflow = false` it drops and counts the entries instead. WAL I/O errors and client updates are retried inside loki.write until the context is cancelled or the component stops, so a failing WAL turns into backpressure instead of an error returned to the source.

### Component rules

Every component in the pipeline follows these rules. Together they keep config reloads and shutdown from deadlocking or losing entries in flight.

1. **Consume() is a plain call.** This applies to every component that exports a Loki receiver, such as `loki.echo`, `loki.relabel`, `loki.enrich`, `loki.secretfilter`, `loki.process`, `loki.write`, `otelcol.receiver.loki` and `database_observability.postgres`.
   - It works from the moment the component is created until it stops. During a config reload, upstream components are pointed at new components before those are started, so a Consume() that waited for the component to start would block the upstream component.
   - It only waits on downstream backpressure. Cancelling the context or stopping the component ends the wait.
   - It returns `nil`, a context error or `ErrConsumerStopped`. The component retries its own failures. Entries it drops on purpose are counted in its metrics, and Consume() returns `nil`. An `ErrConsumerStopped` from downstream is returned to the caller unchanged.
   - After the component stops, new and in-flight calls return `ErrConsumerStopped`. A component that buffers entries stops in this order: mark itself stopped, wait for in-flight calls to return, then flush the buffer.
   - It never holds a lock while calling downstream, so an Update() of the component is never stuck behind a blocked downstream call.
2. **Update() never waits on data.**
   - It switches `FanoutV2` to the new downstream first, before anything that can fail.
   - A source restarts its workers only when the relevant config changed. It cancels their context first, then waits for them to exit.
3. **Sources handle Consume() errors.** Only when Consume() returns an error does a source check `ctx.Err() != nil` to tell a context error apart, instead of inspecting the returned error. A `nil` return is a success, even if the context was cancelled after it.
   - On a context error the entries may not have been delivered. The source does not advance its position and sends them again when the worker restarts. A push source returns the error to its client, so the client retries. A source that can't replay, such as UDP syslog, drops the entries and logs it.
   - On `ErrConsumerStopped` a component in the pipeline has stopped, usually during a config reload. The source does not advance its position and retries with backoff until Consume() succeeds or the context is cancelled. It rebuilds the batch from its own data, since downstream components may have modified it. A push source returns a retryable error to its client instead. A source that can't replay drops the entries and logs it.
   - Any other error is most likely a bug in the pipeline. The source logs it clearly, drops the entries to unblock the pipeline, and advances.
4. **`FanoutV2` delivers to the current downstream and returns its errors**, as described in [FanoutV2](#fanoutv2).

### Error handling and backpressure

Errors propagate synchronously back to the source through Consume() return values. Components retry their own failures, so the only expected errors are a context error and `ErrConsumerStopped` during a config reload. The source has not committed yet (no 200 sent, no positions advanced) when it gets one.

| Error                                            | What happens                                                     | Source behavior                                                                                  |
| ------------------------------------------------ | ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| WAL I/O failure                                  | loki.write retries, Consume() blocks                             | API: request eventually times out → retryable error. File: reader blocks, positions not updated. |
| Queue full, `block_on_overflow = true` (default) | loki.write.Consume() blocks                                      | API: request eventually times out → retryable error. File: reader blocks, positions not updated. |
| Queue full, `block_on_overflow = false`          | loki.write drops the entries, counts them, Consume() returns nil | Same as success. API: respond 200. File: advance position.                                       |
| Context cancelled                                | Consume() returns an error and `ctx.Err() != nil`                | API: return a retryable error so the client retries. File: don't advance, clean exit.            |
| Consumer stopped                                 | Consume() returns `ErrConsumerStopped`                           | API: return a retryable error so the client retries. File: don't advance, retry with backoff.    |
| Unexpected error                                 | Most likely a bug in the pipeline                                | API: log it, respond 500. File: log it, advance position.                                        |

#### Backpressure flow under sustained congestion

1. HTTP send slows down or WAL disk I/O fails
2. loki.write's queue fills up or WAL writes are retried → Consume() blocks
3. Sources can no longer submit:
    * loki.source.api: requests time out → retryable error responses → clients retry, possibly to another instance
    * loki.source.file: file reader blocks → positions not updated

The HTTP request timeout acts as the natural load shedding mechanism. No explicit congestion detection is needed.

### Shutdown

1. Sources stop accepting new requests / reads.
2. In-flight processing is cancelled via context. Since sources have not committed (no 200 response sent, no positions advanced), cancellation is safe: clients will retry, and file positions will be re-read on restart.
3. Components that buffer entries, such as multiline stages in loki.process, flush their buffer downstream. To not lose an entry added by a call that is still in flight, a buffering component stops in this order: mark itself stopped, wait for in-flight calls to return, then flush the buffer.
4. loki.write drains its internal queue (entries already accepted into the queue are flushed to the network / WAL). It already supports this today.

### Config reload

Logs keep flowing while a config reload is applied, so calls can be in flight while upstream components switch to a new downstream and removed components stop. With the [component rules](#component-rules), these are the cases:

- **New component:** `file -> process -> write` becomes `file -> process -> relabel -> write`. Batches in flight during the reload go straight to `write` and skip `relabel`. Nothing is lost. This relies on `relabel` accepting batches as soon as it is created.
- **Component added to fanout:** `process -> write1` becomes `process -> write1, write2`. `write2` misses the batches in flight, `write1` still gets everything.
- **Component removed from fanout:** `process -> write1, write2` becomes `process -> write1`. Batches in flight to `write2` are rejected with `ErrConsumerStopped`. The source retries them, so `write1` gets them twice. Not delivering to a removed component is the intended behavior.
- **Component swapped:** `file -> process -> write` becomes `file -> relabel -> write`. A batch that `file` sent to `process` just before `process` stopped is rejected with `ErrConsumerStopped`. `file` retries it, and by then it forwards to `relabel`, so nothing is lost. The same applies when the swapped component is a sink, for example `write1` becomes `write2`: `process` returns the `ErrConsumerStopped` from `write1` and `file` retries.
- **Component replaced:** `process "a"` becomes `process "b"`. A component with a new name is a new component, so this is the same as a swap. Buffered state, such as multiline entries, is not carried over, it is flushed downstream when `process "a"` stops.
- **Chain removed:** `file -> process -> write "old"` becomes `file -> relabel -> write "new"`. `write "old"` rejects the batch `process` is forwarding, `process` returns `ErrConsumerStopped`, and `file` retries the batch through the new chain.
- **Stale `forward_to`:** a reload changes `file`'s `forward_to` from `write "old"` to `write "new"`, but `file`'s arguments fail to evaluate. Its Update() is not called and it keeps forwarding to `write "old"`, which the reload stops. `file` retries until a fixed config is applied, so nothing is lost, but it stalls until then. If `file` also forwards to a healthy component, that component stalls too and gets the batch again on every retry. The component is reported unhealthy because its evaluation failed.

### Ordering

Some Loki backends, and other possible downstream endpoints, do not accept out-of-order ingestion within a stream. Alloy should therefore keep entries in the order they were read wherever it can.

To discuss this we distinguish between a stream and an origin:

- A **stream** is a set of entries with the same labels, as defined by Loki.
- An **origin** reads log input in sequence, for example a single file read by `loki.source.file` or a single request to `loki.source.api`.

Usually one origin produces one stream, but several origins can write to the same stream. Two files can end up with the same labels, two requests to `loki.source.api` can carry the same labels, or two identical `loki.source.file` components can read the same file.

We guarantee ordering within an origin. The pipeline is called synchronously from the goroutine that reads the origin, and the next batch is only sent once the previous Consume() has returned, so entries from one origin are processed in order. Different origins are processed in parallel.

Ordering is never guaranteed between different origins, even when they write to the same stream. Their entries can reach loki.write in any order.

`loki.process` and `loki.relabel` may change an entry's labels, which can change its final stream identity. Entries from different origins can be relabeled into the same output stream, and there is no ordering between them either. This behavior already exists in the current pipeline design.

### Delivery guarantee

The pipeline prefers duplicates over loss: a downstream component can get the same entries more than once. This can happen when:

- A source is cancelled and Consume() returns an error after part of the batch was already delivered. `loki.source.file` does not advance its position and sends the batch again after it restarts. A push source returns a retryable error and the client sends it again.
- loki.write retries a batch after its client was updated. Endpoints that already accepted the batch get it again.
- A source retries a batch after `ErrConsumerStopped` and the batch was forked on its way. A config reload stops a component on one branch while another branch already accepted the batch, so that branch gets the batches in flight again. For example, `process -> write1, write2` becomes `process -> write1`, and `write1` gets the batches in flight twice.
- A source retries a batch because of a [stale `forward_to`](#config-reload) and also forwards to a healthy component. The healthy component gets the batch again on every retry until a fixed config is applied. For example, `file -> old, kept` should become `file -> new, kept`, but `file`'s arguments fail to evaluate, so `kept` gets the same batch once per retry.

Entries can still be lost in these cases:

- A source gets an unexpected error from Consume(). It logs it, drops the entries and advances (see [Component rules](#component-rules)).
- A source that can't replay, such as UDP syslog, gets a context error. It drops the entries and logs it.
- loki.write has not sent entries from its in-memory queue before an unclean shutdown, or before the drain timeout on shutdown (see [Position file update lag](#position-file-update-lag-optional)).

Entries dropped on purpose, by `block_on_overflow = false` or stages such as `drop`, are counted and are not considered lost.

### Queue sizing

The main tunable queue is in loki.write. We track a metric of the average log entry size observed at runtime. Combined with the available memory on the instance, we can compute a queue capacity with a safety margin to avoid OOM. In first iterations this will be documented as a manual tuning step with some sound defaults.

### Observability

Each stream in a batch records a created timestamp when its entries enter the pipeline. When loki.write sends the entries over the network, it records the total propagation latency in a histogram. This enables alerting on pipeline congestion (a single alert covers both "not reading files fast enough" and general backpressure) and gives users visibility into whether Alloy is keeping up with log volume.

### Position file update lag (optional)

Even with synchronous error propagation, there is a window where entries have been accepted into loki.write's in-memory queue (Consume() returned success) but haven't been sent over the network yet. If Alloy has an unclean shutdown in that time window, those entries are lost but positions were already advanced.

We have an option to mitigate this in the future:

* We introduce a configurable lag for position file updates, initially set to ~30 seconds. We delay committing the read position, so if Alloy crashes, we re-read and re-send entries from the last committed position.
* Loki handles duplicates correctly as long as timestamps come from the log entry itself (not the wall clock), which is the standard behavior.

### Future improvement

#### Position file update lag

Automatically tune the position file update lag based on a total pipeline latency estimate (e.g. from loki.write metrics).

#### Fewer duplicates after a config reload

Some of the duplicates after a config reload can be avoided in `FanoutV2`. When consumers were only removed from the fanout since the snapshot, and every consumer that rejected a batch with `ErrConsumerStopped` is among the removed ones, `FanoutV2` returns `nil` instead of `ErrConsumerStopped`. Not delivering to a removed consumer is the intended behavior, and no consumer was added that should get the batch, so there is nothing for the source to retry. In every other case it returns `ErrConsumerStopped` and the source retries as before.

For example, `process -> write1, write2` becoming `process -> write1` no longer sends the batches in flight to `write1` twice. `process -> write1, write2` becoming `process -> write1, write3` still returns `ErrConsumerStopped`, so `write3` gets the batches in flight and `write1` gets them twice.

This does not help when a consumer was added in the same reload, since it needs the batch. It also does not help when the fork is further down a removed chain, or with a stale `forward_to`, since those fanouts never see their consumers change.
