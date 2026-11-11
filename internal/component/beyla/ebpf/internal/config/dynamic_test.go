//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

const log4shellRule = `
dynamic_instrumentation {
	rule {
		service {
			open_ports = "8080"
		}
		patch {
			class      = "org.apache.logging.log4j.core.lookup.JndiLookup"
			method     = "lookup"
			descriptor = "(Lorg/apache/logging/log4j/core/LogEvent;Ljava/lang/String;)Ljava/lang/String;"
			value      = "none"
			ttl        = "30m"
		}
	}
}
`

func TestDynamicInstrumentationFromAlloySyntaxToBeylaYAML(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(log4shellRule), &args))
	require.NoError(t, args.Validate())

	cfg := buildYAML(t, args, Runtime{Port: 12345})
	di, ok := cfg["dynamic_instrumentation"].(map[string]any)
	require.True(t, ok, "dynamic_instrumentation section must be emitted")
	require.NotContains(t, di, "listen_address", "the rules API must never be switched on from Alloy")
	require.NotContains(t, di, "enabled")

	rules := di["rules"].([]any)
	require.Len(t, rules, 1)
	rule := rules[0].(map[string]any)
	require.Equal(t, []any{map[string]any{"open_ports": "8080"}}, rule["service"])
	patch := rule["patches"].([]any)[0].(map[string]any)
	require.Equal(t, map[string]any{
		"action":     "return",
		"class":      "org.apache.logging.log4j.core.lookup.JndiLookup",
		"descriptor": "(Lorg/apache/logging/log4j/core/LogEvent;Ljava/lang/String;)Ljava/lang/String;",
		"method":     "lookup",
		"ttl":        "1800s",
		"value":      "none",
	}, patch)
}

func TestNoDynamicInstrumentationWhenNoRules(t *testing.T) {
	cfg := buildYAML(t, Arguments{}, Runtime{Port: 12345})
	require.NotContains(t, cfg, "dynamic_instrumentation")
}

func TestEnabledStartsTheEngineWithoutRulesAndWithoutAListener(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte("dynamic_instrumentation {\n\tenabled = true\n}\n"), &args))
	cfg := buildYAML(t, args, Runtime{Port: 12345})
	require.Equal(t, map[string]any{"enabled": true}, cfg["dynamic_instrumentation"])
	require.True(t, args.DynamicInstrumentation.EngineOn())
	require.False(t, Arguments{}.DynamicInstrumentation.EngineOn())
}

func TestPatchTTLFormat(t *testing.T) {
	require.Equal(t, "30s", patchTTL(30*time.Second))
	require.Equal(t, "86400s", patchTTL(24*time.Hour))
	require.Equal(t, "1500ms", patchTTL(1500*time.Millisecond))
}

func TestDynamicInstrumentationValidation(t *testing.T) {
	good := Patch{
		Class: "a.B", Method: "m", Descriptor: "(I)Ljava/lang/String;", Value: "v", TTL: time.Minute,
	}
	svc := Services{{OpenPorts: "8080"}}
	valid := func(mutate func(*DynamicInstrumentation)) error {
		d := DynamicInstrumentation{Rules: []DynamicRule{{Service: svc, Patches: []Patch{good}}}}
		mutate(&d)
		return d.Validate()
	}
	require.NoError(t, valid(func(*DynamicInstrumentation) {}))
	require.NoError(t, DynamicInstrumentation{}.Validate())

	for name, tc := range map[string]struct {
		mutate func(*DynamicInstrumentation)
		want   string
	}{
		"no service":        {func(d *DynamicInstrumentation) { d.Rules[0].Service = nil }, "at least one service"},
		"empty selector":    {func(d *DynamicInstrumentation) { d.Rules[0].Service = Services{{}} }, "must define at least one"},
		"no patch":          {func(d *DynamicInstrumentation) { d.Rules[0].Patches = nil }, "at least one patch"},
		"class with slash":  {func(d *DynamicInstrumentation) { d.Rules[0].Patches[0].Class = "a/B" }, "fully qualified"},
		"method with paren": {func(d *DynamicInstrumentation) { d.Rules[0].Patches[0].Method = "m()" }, "Java method name"},
		"non-string return": {func(d *DynamicInstrumentation) { d.Rules[0].Patches[0].Descriptor = "(I)I" }, "returning java.lang.String"},
		"bad action":        {func(d *DynamicInstrumentation) { d.Rules[0].Patches[0].Action = "throw" }, "not supported"},
		"zero ttl":          {func(d *DynamicInstrumentation) { d.Rules[0].Patches[0].TTL = 0 }, "ttl must be"},
		"ttl over a day":    {func(d *DynamicInstrumentation) { d.Rules[0].Patches[0].TTL = 25 * time.Hour }, "ttl must be"},
		"value too large": {func(d *DynamicInstrumentation) {
			d.Rules[0].Patches[0].Value = string(make([]byte, MaxPatchValueBytes+1))
		}, "maximum is"},
	} {
		t.Run(name, func(t *testing.T) {
			err := valid(tc.mutate)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}

	t.Run("too many patches", func(t *testing.T) {
		d := DynamicInstrumentation{Rules: []DynamicRule{{Service: svc}}}
		for range MaxPatches + 1 {
			d.Rules[0].Patches = append(d.Rules[0].Patches, good)
		}
		require.ErrorContains(t, d.Validate(), "maximum is 256")
	})
}

func TestArgumentsValidateRunsDynamicInstrumentationValidation(t *testing.T) {
	args := Arguments{DynamicInstrumentation: DynamicInstrumentation{Rules: []DynamicRule{{
		Service: Services{{OpenPorts: "8080"}},
		Patches: []Patch{{Class: "a.B", Method: "m", Descriptor: "(I)I", Value: "v", TTL: time.Minute}},
	}}}}
	require.ErrorContains(t, args.Validate(), "returning java.lang.String")
}
