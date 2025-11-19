# Public API to run the agent as a collector receiver

## Meta

- **Author(s)**: @dmathieu
- **Start Date**: September 18 2024
- **Goal End Date**:
- **Primary Reviewers**: @open-telemetry/ebpf-profiler-maintainers

## Problem

As we move forward with the OpenTelemetry Collector being able to handle
profiles, we also want the profiling agent to be able to run as a collector
receiver.
See [PR #87](https://github.com/open-telemetry/opentelemetry-ebpf-profiler/pull/87)

We also intend to provide a collector distribution that will bundle the agent,
which most folks will want to use.

To support use cases where a custom built distribution is needed, we must provide
a stable public API that allows for a receiver to be built and integrated in a distribution.

This design document aims to describe the public API we will be exposing.

## Success Criteria

We define a clear and concise public API for building custom distributions of the collector with ebpf profiling enabled.

### Scope

This document describes what the API will look like.

Like any other receiver (see the
[filelogreceiver](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver)
as an example), there is a single method we need to expose: `NewFactory`, which
returns a [receiver
Factory](https://pkg.go.dev/go.opentelemetry.io/collector/receiver#Factory)
that can handle profiles.

Since the scope of this API is so small, the main question is in which package
the API should be located.

## Proposed Solutions

We will expose the `NewFactory` method as part of a `collector` package within
the ebpf repository, meaning importing and using it will be as follows:

```golang
package main

import (
	"github.com/open-telemetry/opentelemetry-ebpf-profiler/collector"
)

func main() {
	factory := collector.NewFactory()
	// Use the factory
}
```

The intent behind making the factory behind a subpackage as opposed to the root
package of the repository is to provide a better separation of concerns between
packages.

This subpackage would not be its own module. Since the vision is that the agent
should always run as a collector receiver, doing so is probably unnecessary.

## Alternatives

Having the receiver in the root package:

```golang
package main

import (
	agent "github.com/open-telemetry/opentelemetry-ebpf-profiler"
)

func main() {
	factory := agent.NewFactory()
	// Use the factory
}
```

This approach would have the advantage of clarifying that the factory is the
root of the agent running as a collector receiver.
