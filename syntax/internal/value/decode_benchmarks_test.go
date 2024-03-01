package value_test

import (
	"fmt"
	"testing"

	"github.com/grafana/river/internal/value"
)

func BenchmarkObjectDecode(b *testing.B) {
	b.StopTimer()

	// Create a value with 20 keys.
	source := make(map[string]string, 20)
	for i := 0; i < 20; i++ {
		var (
			key   = fmt.Sprintf("key_%d", i+1)
			value = fmt.Sprintf("value_%d", i+1)
		)
		source[key] = value
	}

	sourceVal := value.Encode(source)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		var dst map[string]string
		_ = value.Decode(sourceVal, &dst)
	}
}

func BenchmarkObject(b *testing.B) {
	b.Run("Non-capsule", func(b *testing.B) {
		b.StopTimer()

		vals := make(map[string]value.Value)
		for i := 0; i < 20; i++ {
			vals[fmt.Sprintf("%d", i)] = value.Int(int64(i))
		}

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			_ = value.Object(vals)
		}
	})

	b.Run("Capsule", func(b *testing.B) {
		b.StopTimer()

		vals := make(map[string]value.Value)
		for i := 0; i < 20; i++ {
			vals[fmt.Sprintf("%d", i)] = value.Encapsulate(make(chan int))
		}

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			_ = value.Object(vals)
		}
	})
}

func BenchmarkArray(b *testing.B) {
	b.Run("Non-capsule", func(b *testing.B) {
		b.StopTimer()

		var vals []value.Value
		for i := 0; i < 20; i++ {
			vals = append(vals, value.Int(int64(i)))
		}

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			_ = value.Array(vals...)
		}
	})

	b.Run("Capsule", func(b *testing.B) {
		b.StopTimer()

		var vals []value.Value
		for i := 0; i < 20; i++ {
			vals = append(vals, value.Encapsulate(make(chan int)))
		}

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			_ = value.Array(vals...)
		}
	})
}
