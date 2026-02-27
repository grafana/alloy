package zapadapter_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/util/zapadapter"
)

func Test(t *testing.T) {
	tt := []struct {
		name   string
		field  []zap.Field
		expect string
	}{
		{
			name:   "No fields",
			expect: `level=info msg="Hello, world!"`,
		},
		{
			name:   "Any",
			field:  []zap.Field{zap.Any("key", 12345)},
			expect: `level=info msg="Hello, world!" key=12345`,
		},
		{
			name:   "Bool",
			field:  []zap.Field{zap.Bool("key", true)},
			expect: `level=info msg="Hello, world!" key=true`,
		},
		{
			name:   "Duration",
			field:  []zap.Field{zap.Duration("key", time.Minute)},
			expect: `level=info msg="Hello, world!" key=1m0s`,
		},
		{
			name:   "Error",
			field:  []zap.Field{zap.Error(fmt.Errorf("something went wrong"))},
			expect: `level=info msg="Hello, world!" error="something went wrong"`,
		},
		{
			name:   "Float32",
			field:  []zap.Field{zap.Float32("key", 123.45)},
			expect: `level=info msg="Hello, world!" key=123.45`,
		},
		{
			name:   "Float64",
			field:  []zap.Field{zap.Float64("key", 123.45)},
			expect: `level=info msg="Hello, world!" key=123.45`,
		},
		{
			name:   "Int",
			field:  []zap.Field{zap.Int("key", 12345)},
			expect: `level=info msg="Hello, world!" key=12345`,
		},
		{
			name:   "String",
			field:  []zap.Field{zap.String("key", "foobar")},
			expect: `level=info msg="Hello, world!" key=foobar`,
		},
		{
			name: "Time",
			field: []zap.Field{
				zap.Time("key", time.Date(2022, 12, 1, 1, 1, 1, 1, time.UTC)),
			},
			expect: `level=info msg="Hello, world!" key=2022-12-01T01:01:01.000000001Z`,
		},
		{
			name: "Namespace",
			field: []zap.Field{
				zap.String("key", "foo"),
				zap.Namespace("ns"),
				zap.String("key", "bar"),
			},
			expect: `level=info msg="Hello, world!" key=foo ns.key=bar`,
		},
		{
			name: "Array",
			field: []zap.Field{
				zap.Strings("arr", []string{"foo", "bar"}),
			},
			expect: `level=info msg="Hello, world!" arr="[\"foo\",\"bar\"]"`,
		},
		{
			name: "Object",
			field: []zap.Field{
				zap.Object("obj", testObject{
					obj: map[string]any{
						"foo": "bar",
						"bar": 123,
						"baz": true,
						"qux": map[string]any{
							"foo": "car",
						},
					},
				}),
			},
			expect: `level=info msg="Hello, world!" obj="{\"bar\":123,\"baz\":true,\"foo\":\"bar\",\"qux\":{\"foo\":\"car\"}}"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			inner := log.NewLogfmtLogger(log.NewSyncWriter(&buf))

			zapLogger := zapadapter.New(inner)
			zapLogger.Info("Hello, world!", tc.field...)

			require.Equal(t, tc.expect, strings.TrimSpace(buf.String()))
		})
	}
}

/*
	As of 2025-06-04:

goos: darwin
goarch: arm64
pkg: github.com/grafana/alloy/internal/util/zapadapter
cpu: Apple M2
Benchmark
Benchmark/No_fields_enabled-8         	 1352374	       864.7 ns/op
Benchmark/No_fields_disabled-8        	 6223372	       193.1 ns/op
Benchmark/Any_enabled-8               	 1000000	      1332 ns/op
Benchmark/Any_disabled-8              	 4654744	       240.5 ns/op
Benchmark/Bool_enabled-8              	 1000000	      1015 ns/op
Benchmark/Bool_disabled-8             	 5353936	       253.2 ns/op
Benchmark/Duration_enabled-8          	 1000000	      1062 ns/op
Benchmark/Duration_disabled-8         	 5175646	       238.0 ns/op
Benchmark/Error_enabled-8             	 1000000	      1105 ns/op
Benchmark/Error_disabled-8            	 4905226	       267.0 ns/op
Benchmark/Float32_enabled-8           	 1000000	      1203 ns/op
Benchmark/Float32_disabled-8          	 4813323	       233.6 ns/op
Benchmark/Float64_enabled-8           	 1000000	      1037 ns/op
Benchmark/Float64_disabled-8          	 5130016	       232.8 ns/op
Benchmark/Int_enabled-8               	 1000000	      1065 ns/op
Benchmark/Int_disabled-8              	 5154585	       241.0 ns/op
Benchmark/String_enabled-8            	 1000000	      1025 ns/op
Benchmark/String_disabled-8           	 5105998	       233.3 ns/op
Benchmark/Time_enabled-8              	 1000000	      1143 ns/op
Benchmark/Time_disabled-8             	 4857289	       248.3 ns/op
Benchmark/Array_enabled-8             	  327018	      3529 ns/op
Benchmark/Array_disabled-8            	 4889307	       252.8 ns/op
Benchmark/Object_enabled-8            	  285597	      4187 ns/op
Benchmark/Object_disabled-8           	 4864752	       245.3 ns/op
*/
func Benchmark(b *testing.B) {
	// Benchmark various fields that may be commonly printed.

	runBenchmark(b, "No fields")
	runBenchmark(b, "Any", zap.Any("key", 12345))
	runBenchmark(b, "Bool", zap.Bool("key", true))
	runBenchmark(b, "Duration", zap.Duration("key", time.Second))
	runBenchmark(b, "Error", zap.Error(fmt.Errorf("hello")))
	runBenchmark(b, "Float32", zap.Float32("key", 1234))
	runBenchmark(b, "Float64", zap.Float64("key", 1234))
	runBenchmark(b, "Int", zap.Int("key", 1234))
	runBenchmark(b, "String", zap.String("key", "test"))
	runBenchmark(b, "Time", zap.Time("key", time.Date(2022, 12, 1, 1, 1, 1, 1, time.UTC)))
	runBenchmark(b, "Array", zap.Strings("key", []string{"foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar", "foo", "bar"}))
	runBenchmark(b, "Object", zap.Object("key", testObject{
		obj: map[string]any{
			"foo":  "car",
			"bar":  123,
			"baz":  true,
			"foo2": "bar2",
			"bar2": 123,
			"baz2": true,
			"qux": map[string]any{
				"foo":  "car",
				"bar":  123,
				"baz":  true,
				"foo2": "bar2",
				"bar2": 123,
				"baz2": true,
			},
		},
	}))
}

func runBenchmark(b *testing.B, name string, fields ...zap.Field) {
	innerLogger, err := logging.NewDeferred(io.Discard)
	require.NoError(b, err)
	err = innerLogger.Update(logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
	require.NoError(b, err)

	zapLogger := zapadapter.New(innerLogger)

	b.Run(name+" enabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			zapLogger.Info("Hello, world!", fields...)
		}
	})
	b.Run(name+" disabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			zapLogger.Debug("Hello, world!", fields...)
		}
	})
}

func TestNewWithLevel(t *testing.T) {
	t.Run("suppresses debug logs when level is info", func(t *testing.T) {
		var buf bytes.Buffer
		inner, err := logging.New(&buf, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		zapLogger := zapadapter.NewWithLevel(inner, inner)
		zapLogger.Debug("should not appear")

		require.Empty(t, strings.TrimSpace(buf.String()))
	})

	t.Run("allows debug logs when level is debug", func(t *testing.T) {
		var buf bytes.Buffer
		inner, err := logging.New(&buf, logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		zapLogger := zapadapter.NewWithLevel(inner, inner)
		zapLogger.Debug("should appear")

		require.Contains(t, buf.String(), "should appear")
	})

	t.Run("child logger via With also suppresses debug when level is info", func(t *testing.T) {
		var buf bytes.Buffer
		inner, err := logging.New(&buf, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		child := zapadapter.NewWithLevel(inner, inner).With(zap.String("key", "val"))
		child.Debug("should not appear")

		require.Empty(t, strings.TrimSpace(buf.String()))
	})

	t.Run("reflects hot-reload level change from info to debug", func(t *testing.T) {
		var buf bytes.Buffer
		inner, err := logging.New(&buf, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		zapLogger := zapadapter.NewWithLevel(inner, inner)

		zapLogger.Debug("should not appear before reload")
		require.Empty(t, strings.TrimSpace(buf.String()))

		err = inner.Update(logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		zapLogger.Debug("should appear after reload")
		require.Contains(t, buf.String(), "should appear after reload")
	})

	t.Run("reflects hot-reload level change from debug to error", func(t *testing.T) {
		// This is the primary customer scenario: debug logging was enabled, then
		// the operator reloads to error level and expects debug work to stop.
		var buf bytes.Buffer
		inner, err := logging.New(&buf, logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		zapLogger := zapadapter.NewWithLevel(inner, inner)

		zapLogger.Debug("should appear before reload")
		require.Contains(t, buf.String(), "should appear before reload")
		buf.Reset()

		err = inner.Update(logging.Options{Level: logging.LevelError, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		zapLogger.Debug("should not appear after reload")
		require.Empty(t, strings.TrimSpace(buf.String()))
	})

	t.Run("child logger via With reflects hot-reload", func(t *testing.T) {
		var buf bytes.Buffer
		inner, err := logging.New(&buf, logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		// Create the child logger before the reload.
		child := zapadapter.NewWithLevel(inner, inner).With(zap.String("key", "val"))

		child.Debug("should appear before reload")
		require.Contains(t, buf.String(), "should appear before reload")
		buf.Reset()

		err = inner.Update(logging.Options{Level: logging.LevelError, Format: logging.FormatLogfmt})
		require.NoError(t, err)

		child.Debug("should not appear after reload")
		require.Empty(t, strings.TrimSpace(buf.String()))
	})
}

// TestNewWithLevelAllocations verifies that disabled log levels produce minimal
// allocations with NewWithLevel, confirming zap's early-exit optimisation fires.
//
// When a level is disabled, the only unavoidable allocation is the []Field
// slice that the Go runtime constructs for the variadic ...Field parameter
// before calling Debug/Info/etc.
func TestNewWithLevelAllocations(t *testing.T) {
	makeLogger := func(level logging.Level) (*logging.Logger, *zap.Logger) {
		inner, err := logging.New(io.Discard, logging.Options{Level: level, Format: logging.FormatLogfmt})
		require.NoError(t, err)
		return inner, zapadapter.NewWithLevel(inner, inner)
	}

	t.Run("zero allocs with no fields when debug is disabled", func(t *testing.T) {
		_, lg := makeLogger(logging.LevelInfo)
		allocs := testing.AllocsPerRun(100, func() {
			lg.Debug("msg")
		})
		require.Equal(t, float64(0), allocs)
	})

	t.Run("at most one alloc with scalar fields when debug is disabled", func(t *testing.T) {
		// The single alloc is the []Field variadic slice built by the Go runtime
		// before Debug is called. Field encoding and Write are never reached.
		_, lg := makeLogger(logging.LevelInfo)
		allocs := testing.AllocsPerRun(100, func() {
			lg.Debug("msg", zap.String("k", "v"), zap.Bool("b", true), zap.Int("n", 42))
		})
		require.LessOrEqual(t, allocs, float64(1))
	})

	t.Run("at most one alloc with zap.Any when debug is disabled", func(t *testing.T) {
		// zap.Any with a struct would normally trigger json.Marshal via the
		// fieldEncoder. With NewWithLevel that path is never reached.
		_, lg := makeLogger(logging.LevelInfo)
		allocs := testing.AllocsPerRun(100, func() {
			lg.Debug("msg", zap.Any("obj", struct{ X, Y int }{1, 2}))
		})
		require.LessOrEqual(t, allocs, float64(1))
	})

	t.Run("more than one alloc when debug is disabled without leveler", func(t *testing.T) {
		// Regression baseline: New always reports Enabled()=true, so Write() runs
		// and the full encoding pipeline executes even for suppressed levels.
		inner, err := logging.New(io.Discard, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
		require.NoError(t, err)
		lg := zapadapter.New(inner)
		allocs := testing.AllocsPerRun(100, func() {
			lg.Debug("msg", zap.String("k", "v"), zap.Bool("b", true), zap.Int("n", 42))
		})
		require.Greater(t, allocs, float64(1))
	})

	t.Run("at most one alloc after hot-reload to error", func(t *testing.T) {
		inner, lg := makeLogger(logging.LevelDebug)
		err := inner.Update(logging.Options{Level: logging.LevelError, Format: logging.FormatLogfmt})
		require.NoError(t, err)
		allocs := testing.AllocsPerRun(100, func() {
			lg.Debug("msg", zap.String("k", "v"), zap.Bool("b", true), zap.Int("n", 42))
		})
		require.LessOrEqual(t, allocs, float64(1))
	})

	t.Run("at most one alloc for child logger after hot-reload to error", func(t *testing.T) {
		inner, lg := makeLogger(logging.LevelDebug)
		child := lg.With(zap.String("component", "filter"))
		err := inner.Update(logging.Options{Level: logging.LevelError, Format: logging.FormatLogfmt})
		require.NoError(t, err)
		allocs := testing.AllocsPerRun(100, func() {
			child.Debug("msg", zap.String("k", "v"), zap.Bool("b", true), zap.Int("n", 42))
		})
		require.LessOrEqual(t, allocs, float64(1))
	})
}

type testObject struct {
	obj map[string]any
}

func (o testObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for k, v := range o.obj {
		switch v := v.(type) {
		case string:
			enc.AddString(k, v)
		case int:
			enc.AddInt(k, v)
		case bool:
			enc.AddBool(k, v)
		case map[string]any:
			enc.AddObject(k, &testObject{obj: v})
		default:
			return fmt.Errorf("unsupported type %T", v)
		}
	}
	return nil
}
