//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

// allowlist covers real Beyla keys the published schema doesn't export.
var allowlist = map[string]bool{
	"injector/disable_auto_restart": true,
}

// TestEmittedConfigMatchesSchema builds a maximal config and asserts every emitted
// key exists in Beyla's published schema, so a typo'd or misplaced key — which Beyla
// would silently ignore at runtime — fails here instead. denyUnknownKeys makes the
// schema reject undeclared keys (it sets additionalProperties nowhere). This is a
// key-existence check: the maximal config uses placeholder values, so type and enum
// mismatches are expected and ignored.
func TestEmittedConfigMatchesSchema(t *testing.T) {
	var args Arguments
	fillValue(reflect.ValueOf(&args).Elem(), 0)
	cfg := buildYAML(t, args, Runtime{Port: 12345})

	schema := loadBeylaSchema(t)
	denyUnknownKeys(schema)
	stripPatterns(schema)

	result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema), gojsonschema.NewGoLoader(cfg))
	require.NoError(t, err)

	for _, e := range result.Errors() {
		if e.Type() != "additional_property_not_allowed" {
			continue
		}
		path := fmt.Sprint(e.Details()["property"])
		if field := e.Field(); field != "(root)" {
			path = field + "/" + path
		}
		if !allowlist[path] {
			t.Errorf("emitted key absent from Beyla schema (typo or drift): %s", path)
		}
	}
}

func loadBeylaSchema(t *testing.T) map[string]any {
	t.Helper()
	schemaBytes, err := os.ReadFile("gen/beyla/schema.json")
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(schemaBytes, &schema))
	return schema
}

func TestSupportedSectionsCoverSchema(t *testing.T) {
	var args Arguments
	fillValue(reflect.ValueOf(&args).Elem(), 0)
	cfg := buildYAML(t, args, Runtime{Port: 12345})
	defs := loadBeylaSchema(t)["$defs"].(map[string]any)

	tests := []struct {
		definition string
		path       []string
	}{
		{"EBPFTracer", []string{"ebpf"}},
		{"Buckets", []string{"prometheus_export", "buckets"}},
		{"CloudMetadataConfig", []string{"cloud_metadata"}},
		{"DotnetRuntimeMetricsConfig", []string{"dotnet_runtime_metrics"}},
		{"NameResolverConfig", []string{"name_resolver"}},
		{"ECSNameResolverConfig", []string{"name_resolver", "ecs"}},
	}
	for _, tt := range tests {
		t.Run(tt.definition, func(t *testing.T) {
			emitted := cfg
			for _, key := range tt.path {
				require.Contains(t, emitted, key)
				emitted = emitted[key].(map[string]any)
			}
			properties := defs[tt.definition].(map[string]any)["properties"].(map[string]any)
			for key := range properties {
				require.Contains(t, emitted, key, "Beyla option missing from Alloy: %s.%s", tt.definition, key)
			}
		})
	}
}

func TestMetricFeaturesCoverSchema(t *testing.T) {
	defs := loadBeylaSchema(t)["$defs"].(map[string]any)
	items := defs["Features"].(map[string]any)["items"].(map[string]any)
	var schemaValues []string
	for _, alternative := range items["oneOf"].([]any) {
		for _, value := range alternative.(map[string]any)["enum"].([]any) {
			feature := value.(string)
			schemaValues = append(schemaValues, feature)
			require.True(t, validMetricFeature(feature), "Beyla metric feature missing from Alloy: %s", feature)
		}
	}
	// application_jvm remains accepted by Alloy for backwards compatibility.
	require.ElementsMatch(t, append(schemaValues, "application_jvm"), metricFeatureValues)
}

func TestNameResolverSourcesMatchSchema(t *testing.T) {
	defs := loadBeylaSchema(t)["$defs"].(map[string]any)
	properties := defs["NameResolverConfig"].(map[string]any)["properties"].(map[string]any)
	items := properties["sources"].(map[string]any)["items"].(map[string]any)
	var values []string
	for _, value := range items["enum"].([]any) {
		values = append(values, value.(string))
	}
	require.ElementsMatch(t, values, nameResolverSourceValues)
}

// denyUnknownKeys sets additionalProperties:false on every object node that declares
// properties, so gojsonschema rejects keys the schema doesn't declare.
func denyUnknownKeys(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if props, ok := m["properties"].(map[string]any); ok {
		if _, set := m["additionalProperties"]; !set {
			m["additionalProperties"] = false
		}
		for _, child := range props {
			denyUnknownKeys(child)
		}
	}
	for _, child := range m {
		denyUnknownKeys(child)
	}
}

// stripPatterns removes the "pattern" keyword from every schema node. Some of
// Beyla's upstream patterns (e.g. the log-enricher field-name charset) use
// \uXXXX escapes, which are valid PCRE/JS regex syntax but not the RE2 syntax
// Go's regexp package implements. gojsonschema compiles every "pattern" while
// loading the schema, so one such pattern fails schema loading outright rather
// than just a later match. Dropping "pattern" is safe here: this test only
// checks key existence (see the doc comment above), and pattern mismatches are
// already filtered out of the reported errors.
func stripPatterns(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	delete(m, "pattern")
	for _, child := range m {
		stripPatterns(child)
	}
}
