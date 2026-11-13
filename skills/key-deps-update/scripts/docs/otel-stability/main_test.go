package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAndFormatLevels(t *testing.T) {
	for _, line := range []string{
		"alpha for traces, metrics, and logs",
		"beta for traces and logs, alpha for metrics",
		"beta for traces",
		"alpha for traces to metrics",
		"beta",
	} {
		require.Equal(t, line, formatLevels(parseDocLevels(line)), line)
	}
}

func TestFormatLevelsOrdersSignals(t *testing.T) {
	st := stability{"beta": {"logs", "metrics", "traces"}, "development": {"profiles"}}
	require.Equal(t, "beta for traces, metrics, and logs, development for profiles", formatLevels(st))
	require.Equal(t, "alpha", formatLevels(stability{"alpha": {"extension"}}))
}

func TestExpectedFor(t *testing.T) {
	upstream := stability{"beta": {"traces", "logs"}, "alpha": {"metrics"}, "development": {"profiles"}}

	got := expectedFor(parseDocLevels("beta for traces and logs, alpha for metrics"), upstream)
	require.Equal(t, "beta for traces and logs, alpha for metrics", formatLevels(got))
	require.Equal(t, []string{"profiles (development)"}, unlistedSignals(parseDocLevels("beta for traces and logs, alpha for metrics"), upstream))

	// Upstream moved metrics to beta: the docs line is now stale.
	got = expectedFor(parseDocLevels("beta for traces and logs, alpha for metrics"), stability{"beta": {"traces", "logs", "metrics"}})
	require.Equal(t, "beta for traces, metrics, and logs", formatLevels(got))

	// Extensions document the bare level.
	got = expectedFor(parseDocLevels("beta"), stability{"alpha": {"extension"}})
	require.Equal(t, "alpha", formatLevels(got))
}

func TestHighestLevel(t *testing.T) {
	require.Equal(t, "beta", highestLevel(stability{"alpha": {"metrics"}, "beta": {"traces"}}))
	require.Equal(t, "", highestLevel(stability{"deprecated": {"traces"}}))
}

func TestModuleFromURL(t *testing.T) {
	require.Equal(t,
		"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver",
		moduleFromURL(`https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awss3receiver`))
	require.Equal(t,
		"go.opentelemetry.io/collector/receiver/otlpreceiver",
		moduleFromURL(`https://github.com/open-telemetry/opentelemetry-collector/tree/{{< param "OTEL_VERSION" >}}/receiver/otlpreceiver/`))
	require.Equal(t, "", moduleFromURL("https://example.com/foo"))
}

func TestLinkTarget(t *testing.T) {
	doc := "`x` is a wrapper over the upstream OpenTelemetry Collector [`awss3`][] receiver.\n\n" +
		"[`awss3`]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param \"OTEL_VERSION\" >}}/receiver/awss3receiver\n"
	m := wrapperRe.FindStringSubmatch(doc)
	require.NotNil(t, m)
	require.Equal(t, "`awss3`", m[1])
	require.Equal(t, `https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/{{< param "OTEL_VERSION" >}}/receiver/awss3receiver`, linkTarget(doc, m[1]))
}
