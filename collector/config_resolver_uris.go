package main

import (
	"flag"
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	"github.com/spf13/cobra"
)

// pflagGoFlagWrapper matches github.com/spf13/pflag.flagValueWrapper (field names/types only;
// used to read the wrapped std flag.Value; see golangflag.go).
type pflagGoFlagWrapper struct {
	inner    flag.Value
	flagType string
}

type otelConfigFlagLayout struct {
	values []string
	sets   []string
}

func otelConfigResolverParts(cmd *cobra.Command) (configURIs []string, setURIs []string, err error) {
	if cmd == nil {
		return nil, nil, fmt.Errorf("nil command")
	}
	f := cmd.Flags().Lookup("config")
	if f == nil {
		return nil, nil, fmt.Errorf("no config flag registered on command %q", cmd.Name())
	}
	return extractOtelConfigFlagSlices(f.Value)
}

func extractOtelConfigFlagSlices(v interface{}) (configURIs []string, setURIs []string, err error) {
	if v == nil {
		return nil, nil, fmt.Errorf("nil flag value")
	}
	inner := unwrapPFlagGoFlagValue(v)
	if inner == nil {
		return nil, nil, fmt.Errorf("nil inner config flag value")
	}
	rv := reflect.ValueOf(inner)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return nil, nil, fmt.Errorf("unexpected config flag value type %T (expected non-nil pointer)", inner)
	}
	// otelcol's configFlagValue is an unexported type; read via matching layout.
	layout := (*otelConfigFlagLayout)(unsafe.Pointer(rv.Pointer()))
	return append([]string(nil), layout.values...), append([]string(nil), layout.sets...), nil
}

func unwrapPFlagGoFlagValue(v interface{}) flag.Value {
	cur, ok := v.(flag.Value)
	if !ok || cur == nil {
		return nil
	}
	for range 8 {
		if cur == nil {
			return nil
		}
		rv := reflect.ValueOf(cur)
		if rv.Kind() != reflect.Pointer || rv.IsNil() {
			return cur
		}
		if !strings.Contains(rv.Type().String(), "flagValueWrapper") {
			return cur
		}
		w := (*pflagGoFlagWrapper)(unsafe.Pointer(rv.Pointer()))
		if w.inner == nil || w.inner == cur {
			return cur
		}
		cur = w.inner
	}
	return cur
}
