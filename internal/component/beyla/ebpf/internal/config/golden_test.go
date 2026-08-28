//go:build (linux && arm64) || (linux && amd64)

package config

import (
	"io"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// TestEmittedYAMLGolden locks the full emitted Beyla YAML for a maximally-populated
// config. It guards mechanical refactors of the translation (e.g. the move to
// Convert() methods): the output must stay byte-identical. Regenerate with
// UPDATE_GOLDEN=1 only when an intended translation change is made.
func TestEmittedYAMLGolden(t *testing.T) {
	var args Arguments
	fillValue(reflect.ValueOf(&args).Elem(), 0)
	args.Output = &otelcol.ConsumerArguments{
		Metrics: []otelcol.Consumer{nil},
		Traces:  []otelcol.Consumer{nil},
	}

	rt := Runtime{Port: 12345, HealthAddr: "@beyla-health", OTLPAddr: "@beyla-otlp"}
	got, err := yaml.Marshal(Build(args, rt, slog.New(slog.NewTextHandler(io.Discard, nil))))
	require.NoError(t, err)

	const golden = "testdata/beyla_full_config.golden.yaml"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, got, 0o644))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}

var configPkg = reflect.TypeOf(Arguments{}).PkgPath()
var durationType = reflect.TypeOf(time.Duration(0))

// fillValue sets every field to a non-zero value so Build exercises every
// translation path. Fields whose type lives outside this package (e.g. Output's
// otelcol consumer) are skipped — they aren't part of the Beyla YAML.
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
