# Pyroscope to OTLP integration test

Run with the Docker integration runner from `integration-tests/docker`:

```sh
go run . --test pyroscope-otlp
```

The test discovers processes, keeps only the `profile-workload` executable,
derives its service name, collects actual Linux eBPF profiles, and verifies
positive CPU samples and the workload's function at the OTLP sink. It checks
the entire discovery, relabeling, collection, conversion, and export path.

The Alloy container runs privileged in the host PID namespace and mounts the
kernel tracing filesystem, as required by `pyroscope.ebpf`. The Docker host
must support eBPF profiling. This is a Linux integration test, separate from
the portable scrape and conversion pipeline tests.

Run the portable source-to-export tests from the repository root:

```sh
go test -race -tags=nodocker -run TestProfilesScrapePipeline ./internal/component/otelcol/receiver/pyroscope
```

These tests scrape a known pprof fixture and a live Go CPU profile, then verify
the sample values, workload functions, service labels, and instrumentation
scope after OTLP gRPC export. They don't require Docker or eBPF privileges.
