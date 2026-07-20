//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// TestSchemaCoverage flags Beyla schema options not exposed by Alloy; the accepted
// set is snapshotted in testdata/unexposed_schema.txt (UPDATE_COVERAGE=1 to refresh).
func TestSchemaCoverage(t *testing.T) {
	var args Arguments
	fillValue(reflect.ValueOf(&args).Elem(), 0)
	args.Output = &otelcol.ConsumerArguments{
		Metrics: []otelcol.Consumer{nil},
		Traces:  []otelcol.Consumer{nil},
	}
	emitted := buildYAML(t, args, Runtime{Port: 12345, HealthAddr: "@beyla-health", OTLPAddr: "@beyla-otlp"})

	data, err := os.ReadFile("gen/beyla/schema.json")
	require.NoError(t, err)
	var doc schemaDoc
	require.NoError(t, json.Unmarshal(data, &doc))

	var unexposed []string
	coverageWalk(doc.Defs, &schemaNode{Properties: doc.Properties}, emitted, "", &unexposed)
	sort.Strings(unexposed)
	got := strings.Join(unexposed, "\n") + "\n"

	const snap = "testdata/unexposed_schema.txt"
	if os.Getenv("UPDATE_COVERAGE") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(snap, []byte(got), 0o644))
	}

	want, err := os.ReadFile(snap)
	require.NoError(t, err)
	require.Equal(t, string(want), got,
		"unexposed Beyla schema options changed: expose the new ones (Arguments field + Convert()) or run UPDATE_COVERAGE=1 to accept them")
}

func coverageWalk(defs map[string]*schemaNode, n *schemaNode, emitted any, path string, unexposed *[]string) {
	n = resolveRef(defs, n)
	if n == nil {
		return
	}

	switch v := emitted.(type) {
	case map[string]any:
		keys := make([]string, 0, len(n.Properties))
		for k := range n.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if cv, ok := v[k]; ok {
				coverageWalk(defs, n.Properties[k], cv, path+"/"+k, unexposed)
			} else {
				*unexposed = append(*unexposed, strings.TrimPrefix(path+"/"+k, "/"))
			}
		}
		if ap := additionalPropsSchema(n); ap != nil {
			for _, cv := range v {
				coverageWalk(defs, ap, cv, path+"/*", unexposed)
				break
			}
		}
	case []any:
		if len(v) > 0 && n.Items != nil {
			coverageWalk(defs, n.Items, v[0], path, unexposed)
		}
	}
}
