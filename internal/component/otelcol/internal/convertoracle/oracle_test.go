package convertoracle_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/processor/batchprocessor"
)

type reporter struct {
	path  cmp.Path
	diffs []string
}

func (r *reporter) PushStep(ps cmp.PathStep) { r.path = append(r.path, ps) }
func (r *reporter) PopStep()                 { r.path = r.path[:len(r.path)-1] }
func (r *reporter) Report(rs cmp.Result) {
	if rs.Equal() {
		return
	}
	vx, vy := r.path.Last().Values()
	var sb strings.Builder
	for _, s := range r.path {
		switch s := s.(type) {
		case cmp.StructField:
			sb.WriteString("." + s.Name())
		case cmp.MapIndex:
			fmt.Fprintf(&sb, "[%v]", s.Key())
		case cmp.SliceIndex:
			fmt.Fprintf(&sb, "[%d]", s.Key())
		}
	}
	str := func(v reflect.Value) string {
		if !v.IsValid() {
			return "<absent>"
		}
		if v.CanInterface() {
			return fmt.Sprintf("%+v", v.Interface())
		}
		return fmt.Sprintf("%+v", v)
	}
	r.diffs = append(r.diffs, fmt.Sprintf("%s (collector=%s alloy=%s)", strings.TrimPrefix(sb.String(), "."), str(vx), str(vy)))
}

func diff(want, got otelcomponent.Config) []string {
	rep := &reporter{}
	cmp.Equal(normalize(deref(want)), normalize(deref(got)),
		cmp.Exporter(func(reflect.Type) bool { return true }),
		cmpopts.EquateEmpty(),
		cmp.Reporter(rep))
	return rep.diffs
}

func deref(v any) any {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}
	return rv.Interface()
}

type factory interface {
	CreateDefaultConfig() otelcomponent.Config
}

type converter interface {
	Convert() (otelcomponent.Config, error)
}

type tc struct {
	name    string
	alloy   string
	collect map[string]any
	// Extra assertion on the collector-side result, evaluated after the diff.
	check func(t *testing.T, got otelcomponent.Config)
}

func run[T any](t *testing.T, f factory, cases []tc) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var args T
			require.NoError(t, syntax.Unmarshal([]byte(c.alloy), &args))
			got, err := any(args).(converter).Convert()
			require.NoError(t, err)

			want := f.CreateDefaultConfig()
			require.NoError(t, confmap.NewFromStringMap(c.collect).Unmarshal(want))

			require.Empty(t, diff(want, got))
			if c.check != nil {
				c.check(t, got)
			}
		})
	}
}

func TestBatch(t *testing.T) {
	// send_batch_size and send_batch_max_size carry declared divergences (batch.spec.yaml),
	// so the collector side spells Alloy's defaults out.
	run[batch.Arguments](t, batchprocessor.NewFactory(), []tc{
		{name: "minimal", alloy: `output {}`, collect: map[string]any{"send_batch_size": 2000, "send_batch_max_size": 3000}},
		{name: "send_batch_size", alloy: `
			send_batch_size = 100
			output {}`, collect: map[string]any{"send_batch_size": 100, "send_batch_max_size": 3000}},
		{name: "explicit upstream default survives", alloy: `
			send_batch_size = 8192
			send_batch_max_size = 0
			output {}`, collect: map[string]any{"send_batch_size": 8192, "send_batch_max_size": 0}},
		{name: "all", alloy: `
			timeout = "1s"
			send_batch_size = 10
			send_batch_max_size = 20
			metadata_keys = ["a"]
			metadata_cardinality_limit = 5
			output {}`, collect: map[string]any{"timeout": "1s", "send_batch_size": 10, "send_batch_max_size": 20, "metadata_keys": []any{"a"}, "metadata_cardinality_limit": 5}},
	})
}

func TestNegativeControl(t *testing.T) {
	a := batchprocessor.NewFactory().CreateDefaultConfig()
	b := batchprocessor.NewFactory().CreateDefaultConfig()
	b.(*batchprocessor.Config).SendBatchSize = 1
	require.NotEmpty(t, diff(a, b))
}
