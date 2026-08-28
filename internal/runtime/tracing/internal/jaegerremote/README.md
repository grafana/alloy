# jaegerremote

This package contains a temporary fork of
the `go.opentelemetry.io/contrib/samplers/jaegerremote@v0.5.2` module which is
used to work around an issue where importing the OpenTelemetry Collector Jaeger
receiver and jaegerremote modules causes a run-time panic.

See [open-telemetry/opentelemetry-go-contrib#2981][upstream-issue] for tracking
the issue that led to the need for this fork.

[upstream-issue]: (https://github.com/open-telemetry/opentelemetry-go-contrib/issues/2981)
