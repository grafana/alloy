# Proposal: Reliable Loki pipelines

- Author: Karl Persson (@kalleep), Piotr Gwizdala (@thampiotr)
- Last updated: 2026-03-09
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

**Our default behavior will follow the all-or-nothing model.** The overall operation succeeds only if all downstream components succeed. This provides clear and predictably safe semantics by default.

**Option 4** can be supported with already existing `block_on_overflow`. If this is configured for endpoint logs would be dropped and not reported as error and the specific endpoint would not be able to bottleneck the pipeline.

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

### Interface

This is for illustration purposes only. The actual naming may be different.

```go
type Consumer interface {
  Consume(ctx context.Context, entries []loki.Entry) error
}
```

This replaces the current Chan() chan<- loki.Entry. The call is synchronous — the caller's goroutine does the work through the pipeline. context.Context scopes the processing lifetime (HTTP request context, shutdown signal). error propagates failures back to the source.

Not every Loki backend (or other downstream endpoint) has out-of-order ingestion enabled, so entries within a single stream must be processed in order. To achieve parallelism without breaking ordering, we assign entire streams to specific goroutines — all entries for a given stream always go to the same goroutine (see ShardingConsumer below).

Pipeline components like loki.process, loki.relabel, loki.write, and fan-out all implement Consumer. These components are stream-agnostic — they process whatever entries they receive and do not perform sharding. The entries they receive will already be grouped by stream.

**Note on label mutation**: loki.process and loki.relabel can change an entry's labels, which changes its stream identity. This is safe because sharding happens before these components run, based on the original stream labels. All entries from the same original stream are on the same worker and processed in order — this ordering is preserved through mutations. When entries reach loki.write, it reshards internally based on final labels. If entries from different original streams end up in the same target stream after relabeling, their interleaving is fine — they had no ordering relationship before relabeling.

The existing loki.Entry already carries `model.LabelSet` which identifies the stream, so no changes to the entry type are needed.

### ShardingConsumer

Source components (loki.source.api, loki.source.file) receive entries that may belong to multiple streams. A ShardingConsumer sits at the boundary between source and pipeline. It groups entries by stream, dispatches each group to a worker goroutine (by stream hash), and waits for all workers to complete. Each worker calls a plain Consumer chain (e.g. loki.process → loki.write) with entries from a single stream.

```go
type ShardingConsumer struct { ... }

// Consume groups entries by stream hash, dispatches to workers, and waits
// for all to complete. Returns error if any stream's processing failed.
func (s *ShardingConsumer) Consume(ctx context.Context, entries []loki.Entry) error
```

### FanoutConsumer

Every component that can fan-out should use `FanoutConsumer`. This one is responsible to call all `Consume` on all consumers a component should forward to. From sources we would pass this one to `ShardingConsumer`.

```go
type FanoutConsumer struct { ... }

// Consume calls consume on all internal consumers pass to it and aggregate errors.
func (f *FanoutConsumer) Consume(ctx context.Context, entries []loki.Entry) error
```

### Architecture

```
loki.source.api                            loki.source.file
     |                                          |
     | HTTP handler receives request            | File reader reads entries
     |                                          | (one stream per target)
     v                                          v
+------------------------------------------------------------+
|              ShardingConsumer                              |
|                                                            |
|  Groups entries by stream hash, dispatches to N workers.   |
|  WAITS for all workers to complete, returns result.        |
|                                                            |
|  worker 0 --+                                              |
|  worker 1 --+-- Consume(logs belonging to this worker)     |
|  worker 2 --+           |                                  |
|  ...        |           |                                  |
|  worker N --+           |                                  |
+-------------------------+----------------------------------+
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
|  returns error on WAL I/O failure or                       |
|  blocks if in-memory queue is full for backpressure        |
+------------------------------------------------------------+
```

ShardingConsumer runs N workers which process entries from consistently hashed streams. So entries from the same stream go to the same worker and are processed in order. Different streams are processed concurrently across workers.

Each worker handles the full processing path inline: Consume() called by the worker passes through loki.process (mutations, filtering) and into loki.write. No goroutine hand-offs. This keeps goroutine stacks under control and avoids context switching between pools.

loki.source.api. Calls shardingConsumer.Consume(ctx, entries) with all entries from the HTTP request. If success → respond 200. If error → respond with a retryable status code (429/503). If the client disconnects, the HTTP request context is cancelled and workers abort — no 200 is sent, so no commitment is made. To bound goroutine creation, loki.source.api will limit the number of concurrently accepted connections at the HTTP server level.

loki.source.file. Calls shardingConsumer.Consume(ctx, entries) for each batch read from a file target (already single-stream). If success → advance position. If error → do not advance, will retry. If context is cancelled (shutdown) → do not advance, clean exit.

loki.write. Implements Consumer by appending to a WAL or in-memory queue. Blocks if queue is full; returns error on WAL I/O failure or context cancellation. WAL I/O errors are retryable — another Alloy instance may have a healthy WAL, or the error may be transient.

### Error handling and backpressure

Errors propagate synchronously back to the source through Consume() return values. The source has not committed yet (no 200 sent, no positions advanced), so it can always handle errors safely.

| Error                   | What happens                                     | Source behavior                                                                                  |
| ----------------------- | ------------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| WAL I/O failure         | loki.write.Consume() returns retryable error     | API: respond 429/503. File: don't advance position. Both retry.                                  |
| Queue full (congestion) | loki.write.Consume() blocks                      | API: request eventually times out → retryable error. File: reader blocks, positions not updated. |
| Context cancelled       | loki.write.Consume() returns ctx.Err()           | API: no response needed (client gone). File: don't advance, clean exit.                          |
| Fan-out partial failure | Overall Consume() returns error (all-or-nothing) | Same as WAL error — retryable response or no position advance.                                   |

### Backpressure flow under sustained congestion:

1. HTTP send slows down or WAL disk I/O fails
2. loki.write's queue fills up → Consume() blocks
3. Pool workers block → pool input queues fill up
4. Sources can no longer submit:
    * loki.source.api: requests time out → retryable error responses → clients retry, possibly to another instance
    * loki.source.file: file reader blocks → positions not updated

The HTTP request timeout acts as the natural load shedding mechanism. No explicit congestion detection is needed.

### Shutdown

1. Sources stop accepting new requests / reads.
2. In-flight processing is cancelled via context. Since sources have not committed (no 200 response sent, no positions advanced), cancellation is safe — clients will retry, and file positions will be re-read on restart.
3. loki.write drains its internal queue (entries already accepted into the queue are flushed to the network / WAL). This is the only component that needs full draining, and it already supports this today.

### Queue sizing

The main tunable queue is in loki.write (the worker input queues are small and fixed). We track a metric of the average log entry size observed at runtime. Combined with the available memory on the instance, we can compute a queue capacity with a safety margin to avoid OOM. In first iterations this will be documented as a manual tuning step with some sound defaults.

### Observability

Every batch created timestamp when it enters the pipeline. When loki.write sends the entries over the network, it records the total propagation latency in a histogram. This enables alerting on pipeline congestion (a single alert covers both "not reading files fast enough" and general backpressure) and gives users visibility into whether Alloy is keeping up with log volume.

### Position file update lag (optional)

Even with synchronous error propagation via ShardingConsumer, there is a window where entries have been accepted into loki.write's in-memory queue (Consume() returned success) but haven't been sent over the network yet. If Alloy has an unclean shutdown in that time window, those entries are lost but positions were already advanced.

We have an option to mitigate this in the future:

* We introduce a configurable lag for position file updates, initially set to ~30 seconds. We delay committing the read position, so if Alloy crashes, we re-read and re-send entries from the last committed position.
* Loki handles duplicates correctly as long as timestamps come from the log entry itself (not the wall clock), which is the standard behavior.

### Future improvement:

Automatically tune the lag based on a total pipeline latency estimate (e.g. from loki.write metrics).
