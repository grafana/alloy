package zapadapter_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

			zapLogger := zapadapter.New(inner, nil)
			zapLogger.Info("Hello, world!", tc.field...)

			require.Equal(t, tc.expect, strings.TrimSpace(buf.String()))
		})
	}
}

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
	runBenchmark(b, "Array", zap.Strings("key", []string{"foo", "bar"}))
	runBenchmark(b, "Object", zap.Object("key", testObject{
		obj: map[string]any{
			"foo": "bar",
			"bar": 123,
			"baz": true,
			"qux": map[string]any{
				"foo": "car",
			},
		},
	}))
}

func runBenchmark(b *testing.B, name string, fields ...zap.Field) {
	innerLogger := log.NewLogfmtLogger(io.Discard)
	innerLogger = level.NewFilter(innerLogger, level.AllowAll())

	zapLogger := zapadapter.New(innerLogger, nil)

	b.Run(name, func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			zapLogger.Info("Hello, world!", fields...)
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
