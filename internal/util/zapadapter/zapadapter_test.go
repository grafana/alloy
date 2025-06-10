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
