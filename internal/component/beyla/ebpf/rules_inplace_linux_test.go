//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/config"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
	"github.com/grafana/alloy/internal/component/otelcol"
)

func patch(value string, ttl time.Duration) config.Patch {
	return config.Patch{
		Class: "org.apache.logging.log4j.core.lookup.JndiLookup", Method: "lookup",
		Descriptor: "(Lorg/apache/logging/log4j/core/LogEvent;Ljava/lang/String;)Ljava/lang/String;",
		Value:      value, TTL: ttl,
	}
}

func argsWithPatches(patches ...config.Patch) Arguments {
	args := Arguments{
		Discovery: config.Discovery{Instrument: config.Services{{OpenPorts: "8080"}}},
		Metrics:   config.Metrics{Features: []string{"application"}},
	}
	if len(patches) > 0 {
		args.DynamicInstrumentation = config.DynamicInstrumentation{Rules: []config.DynamicRule{{
			Service: config.Services{{OpenPorts: "8080"}},
			Patches: patches,
		}}}
	}
	return args
}

// runningComponent returns a Component as if Beyla was running with args: its config memfd is written
// and the subprocess "cancel" function records whether the subprocess was stopped.
func runningComponent(t *testing.T, args Arguments) (c *Component, configPath string, stopped *bool) {
	t.Helper()
	c = &Component{
		opts:         component.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
		args:         args,
		argsUpdate:   make(chan Arguments, 1),
		subprocess:   subprocess.New(),
		restartTimer: time.NewTimer(time.Hour),
	}
	c.subprocess.SetListen(12345, "http://127.0.0.1:12345")
	c.subprocess.SetHealthAddr("@test-health")

	path, closeFn, err := c.writeConfigFile()
	require.NoError(t, err)
	t.Cleanup(closeFn)
	c.subprocess.SetConfig(path, closeFn)

	stopped = new(bool)
	c.subprocessCancel = func() { *stopped = true }
	return c, path, stopped
}

func readRules(t *testing.T, path string) []any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(data, &doc), "the rewritten config must be clean YAML, with no leftover bytes")
	di, ok := doc["dynamic_instrumentation"].(map[string]any)
	if !ok {
		return nil
	}
	return di["rules"].([]any)
}

func firstPatch(rules []any) map[string]any {
	return rules[0].(map[string]any)["patches"].([]any)[0].(map[string]any)
}

func TestRuleChangesAreWrittenInPlaceWithoutRestart(t *testing.T) {
	c, path, stopped := runningComponent(t, argsWithPatches(patch("first-value-is-quite-long", 30*time.Minute)))
	require.Equal(t, "first-value-is-quite-long", firstPatch(readRules(t, path))["value"])

	// Change a patch to a SHORTER document: no bytes of the old file may survive.
	next := argsWithPatches(patch("v2", time.Hour))
	c.handleArgsUpdate(next)
	require.False(t, *stopped, "a rule-only change must not stop Beyla")
	require.Equal(t, next, c.args)
	require.Equal(t, "v2", firstPatch(readRules(t, path))["value"])
	require.Equal(t, "3600s", firstPatch(readRules(t, path))["ttl"])

	// Remove every rule.
	c.handleArgsUpdate(argsWithPatches())
	require.False(t, *stopped)
	require.Empty(t, readRules(t, path))

	// Add one back.
	c.handleArgsUpdate(argsWithPatches(patch("again", time.Minute)))
	require.False(t, *stopped)
	require.Equal(t, "again", firstPatch(readRules(t, path))["value"])
}

func TestOtherChangesStillRestartBeyla(t *testing.T) {
	c, path, _ := runningComponent(t, argsWithPatches(patch("a", time.Minute)))
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	other := argsWithPatches(patch("a", time.Minute))
	other.Discovery.Instrument = config.Services{{OpenPorts: "9090"}}

	require.False(t, c.applyRulesInPlace(other), "a discovery change needs a restart")
	other = argsWithPatches(patch("b", time.Minute))
	other.LogLevel = "debug"
	require.False(t, c.applyRulesInPlace(other), "a rule change combined with another change needs a restart")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after), "a refused in-place update must not touch the config file")
}

func TestFirstRuleNeedsARestartUnlessTheEngineWasStarted(t *testing.T) {
	// Started without rules and without `enabled`: Beyla has no rule engine, so adding a rule restarts it.
	c, path, _ := runningComponent(t, argsWithPatches())
	require.False(t, c.ruleEngineOn)
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.False(t, c.applyRulesInPlace(argsWithPatches(patch("a", time.Minute))))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))

	// Started with `enabled = true` and no rules: the first rule goes in place.
	started := argsWithPatches()
	started.DynamicInstrumentation.Enabled = true
	c, path, stopped := runningComponent(t, started)
	require.True(t, c.ruleEngineOn)
	next := argsWithPatches(patch("a", time.Minute))
	next.DynamicInstrumentation.Enabled = true
	c.handleArgsUpdate(next)
	require.False(t, *stopped)
	require.Equal(t, "a", firstPatch(readRules(t, path))["value"])

	// Flipping `enabled` itself is not a rule change.
	flipped := argsWithPatches(patch("a", time.Minute))
	require.False(t, c.applyRulesInPlace(flipped))

	// Started with a rule, all rules removed, one added back: still in place (the engine kept running).
	c, path, stopped = runningComponent(t, argsWithPatches(patch("a", time.Minute)))
	c.handleArgsUpdate(argsWithPatches())
	require.False(t, *stopped)
	c.handleArgsUpdate(argsWithPatches(patch("b", time.Minute)))
	require.False(t, *stopped)
	require.Equal(t, "b", firstPatch(readRules(t, path))["value"])
}

func TestNoInPlaceUpdateWithoutARunningConfig(t *testing.T) {
	c, _, _ := runningComponent(t, argsWithPatches(patch("a", time.Minute)))
	c.configFD = 0
	require.False(t, c.applyRulesInPlace(argsWithPatches(patch("b", time.Minute))))

	c, _, _ = runningComponent(t, argsWithPatches(patch("a", time.Minute)))
	c.subprocessCancel = nil
	require.False(t, c.applyRulesInPlace(argsWithPatches(patch("b", time.Minute))))
}

type fakeConsumer struct{ otelcol.Consumer }

func TestSameConsumers(t *testing.T) {
	a, b := &fakeConsumer{}, &fakeConsumer{}
	require.True(t, sameConsumers(nil, nil))
	require.False(t, sameConsumers(nil, &otelcol.ConsumerArguments{}))
	require.True(t, sameConsumers(&otelcol.ConsumerArguments{Traces: []otelcol.Consumer{a}}, &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{a}}))
	require.False(t, sameConsumers(&otelcol.ConsumerArguments{Traces: []otelcol.Consumer{a}}, &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{b}}))
}
