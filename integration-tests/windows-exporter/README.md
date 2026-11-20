# Windows exporter integration test

These tests start the Alloy binary on a Windows host with `prometheus.exporter.windows` configs
and check the metrics that the component serves. They need no Docker, Mimir, or installer.

The Docker and Kubernetes suites run Linux containers, so they can't run this exporter.

**Run**

From the repository root on Windows:

```bash
make integration-test-windows-exporter
```

The target builds Alloy to `build/alloy.exe` first. To use a binary you already built (for example with `make alloy ALLOY_BINARY=build/alloy.exe`), run the tests directly:

```bash
cd integration-tests/windows-exporter
ALLOY_BINARY=../../build/alloy.exe go test -v -count=1 -tags="gore2regex alloyintegrationtests" ./...
```

**Add a scenario**

1. Add `configs/<name>.alloy` with a `prometheus.exporter.windows "test"` component.
   Use `{{PROBE_GLOB}}` for a path to a temporary file.
2. Add a test that calls `startAlloy` and asserts on the metrics or the logs.
