# OTLP Profiles integration test

Run this test with the Docker integration runner from `integration-tests/docker`:

```sh
go run . --test otlp-profiles
```

The test sends Profiles over OTLP gRPC, HTTP protobuf, and HTTP JSON through this
pipeline:

```text
OTLP receiver
  -> memory limiter -> OTLP gRPC exporter -> sink
  -> OTLP gRPC exporter                   -> sink-direct
```

Both destinations must receive the complete original payload for every transport.
The fixtures include sample values, profile IDs, timestamps, attributes, stacks,
function names, and source lines. Sending queues and retries are disabled so
export failures are visible immediately.

Separate one-shot Alloy containers verify that CLI startup rejects Profiles at
the default and public-preview stability levels. A timeout or an unrelated startup
error fails the test. The working pipeline uses `--stability.level=experimental`.

This test covers OTLP routing, processor forwarding, and fan-out. It doesn't cover
pprof conversion, Pyroscope storage, queue persistence, retry recovery, or load.
