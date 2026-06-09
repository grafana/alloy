//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestEmittedYAMLMatchesSchema is the correctness net for the hand-written
// Args->Beyla-YAML translation: it fully populates an Arguments, emits the YAML,
// and asserts every emitted key exists at the right place in Beyla's published
// schema (gen/schema.json). A typo'd or misplaced key -- which Beyla would
// silently ignore at runtime -- fails the test here instead.
//
// The walk is deliberately stricter than JSON-Schema semantics: the schema sets
// additionalProperties nowhere, so a stock validator would accept unknown keys.
// We treat a key that is neither a declared property nor covered by an explicit
// additionalProperties schema (a genuine dynamic map) as a failure.
func TestEmittedYAMLMatchesSchema(t *testing.T) {
	data, err := os.ReadFile("gen/schema.json")
	require.NoError(t, err)

	var doc schemaDoc
	require.NoError(t, json.Unmarshal(data, &doc))
	root := &schemaNode{Properties: doc.Properties}

	var args Arguments
	fillValue(reflect.ValueOf(&args).Elem(), 0)

	cfg := buildYAML(t, args, Runtime{Port: 12345})

	var unknown []string
	walkSchema(doc.Defs, cfg, root, "", &unknown)

	unknown = filterAllowlisted(unknown)
	require.Empty(t, unknown,
		"emitted YAML keys absent from Beyla schema (typo or drift):\n  %s",
		strings.Join(unknown, "\n  "))
}

// allowlist holds emitted key paths that are legitimately absent from the schema
// (real Beyla keys the schema doesn't export). Keep this list short and justified.
var allowlist = map[string]bool{
	// Real Beyla injector key, present in Beyla's own config examples but not
	// exported into schema.json.
	"injector/disable_auto_restart": true,
}

func filterAllowlisted(paths []string) []string {
	out := paths[:0]
	for _, p := range paths {
		if !allowlist[p] {
			out = append(out, p)
		}
	}
	return out
}

// ── schema model ────────────────────────────────────────────────────────────

type schemaDoc struct {
	Defs       map[string]*schemaNode `json:"$defs"`
	Properties map[string]*schemaNode `json:"properties"`
}

type schemaNode struct {
	Ref                  string                 `json:"$ref"`
	Properties           map[string]*schemaNode `json:"properties"`
	Items                *schemaNode            `json:"items"`
	AdditionalProperties json.RawMessage        `json:"additionalProperties"`
}

func resolveRef(defs map[string]*schemaNode, n *schemaNode) *schemaNode {
	for n != nil && n.Ref != "" {
		n = defs[strings.TrimPrefix(n.Ref, "#/$defs/")]
	}
	return n
}

// additionalPropsSchema returns the value schema for a dynamic-key map, or nil
// when additionalProperties is unset or a boolean.
func additionalPropsSchema(n *schemaNode) *schemaNode {
	if len(n.AdditionalProperties) == 0 || n.AdditionalProperties[0] != '{' {
		return nil
	}
	var s schemaNode
	if json.Unmarshal(n.AdditionalProperties, &s) != nil {
		return nil
	}
	return &s
}

func walkSchema(defs map[string]*schemaNode, val any, n *schemaNode, path string, unknown *[]string) {
	n = resolveRef(defs, n)
	if n == nil {
		return
	}

	switch v := val.(type) {
	case map[string]any:
		ap := resolveRef(defs, additionalPropsSchema(n))
		for k, cv := range v {
			child := resolveRef(defs, n.Properties[k])
			switch {
			case child != nil:
				walkSchema(defs, cv, child, path+"/"+k, unknown)
			case ap != nil:
				walkSchema(defs, cv, ap, path+"/"+k, unknown)
			case len(n.Properties) == 0:
				// free-form object (e.g. map[string]string) — no declared keys, allow.
			default:
				*unknown = append(*unknown, strings.TrimPrefix(path+"/"+k, "/"))
			}
		}
	case []any:
		if item := resolveRef(defs, n.Items); item != nil {
			for _, e := range v {
				walkSchema(defs, e, item, path+"/[]", unknown)
			}
		}
	}
}

// ── reflective population ───────────────────────────────────────────────────

var configPkg = reflect.TypeOf(Arguments{}).PkgPath()
var durationType = reflect.TypeOf(time.Duration(0))

// fillValue sets every field to a non-zero value so Build exercises every
// translation path. Struct fields whose type lives outside this package (e.g.
// Output's otelcol consumer) are skipped — they aren't part of the Beyla YAML.
func fillValue(v reflect.Value, depth int) {
	if depth > 20 {
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillValue(v.Elem(), depth+1)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() || externalStruct(f.Type) {
				continue
			}
			fillValue(v.Field(i), depth+1)
		}
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		fillValue(s.Index(0), depth+1)
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMapWithSize(v.Type(), 1)
		key := reflect.New(v.Type().Key()).Elem()
		fillValue(key, depth+1)
		val := reflect.New(v.Type().Elem()).Elem()
		fillValue(val, depth+1)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Type() == durationType {
			v.SetInt(int64(time.Second))
		} else {
			v.SetInt(1)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	}
}

// externalStruct reports whether t (after unwrapping pointers/collections) is a
// struct defined outside this config package — those aren't translated to YAML.
func externalStruct(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice ||
		t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	pp := t.PkgPath()
	return pp != "" && pp != configPkg
}
