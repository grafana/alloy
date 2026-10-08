package benchdiff

import (
	"fmt"
	"testing"
)

// syntheticResults returns n benchmarks with 10 samples each and sec/op
// multiplied by scale.
func syntheticResults(n int, scale float64) results {
	res := results{cpu: "Test CPU", samples: map[benchKey]map[string][]float64{}}
	for i := range n {
		key := benchKey{pkg: "github.com/grafana/alloy/internal/demo", name: fmt.Sprintf("Demo/case_%d-16", i)}
		units := map[string][]float64{}
		for j := range 10 {
			jitter := 1 + float64(j%3-1)*0.001
			units["sec/op"] = append(units["sec/op"], float64(1000+i)*1e-9*scale*jitter)
			units["allocs/op"] = append(units["allocs/op"], float64(i%7))
		}
		res.samples[key] = units
	}
	return res
}

func BenchmarkCompareUnit(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("benchmarks=%d", n), func(b *testing.B) {
			base, head := syntheticResults(n, 1), syntheticResults(n, 1.1)
			for b.Loop() {
				compareUnit(base, head, "sec/op")
			}
		})
	}
}

func BenchmarkRenderReport(b *testing.B) {
	in := reportInput{
		base:     syntheticResults(100, 1),
		head:     syntheticResults(100, 1.1),
		packages: []Package{{ImportPath: "github.com/grafana/alloy/internal/demo", InBase: true, InHead: true}},
		count:    10,
	}
	for b.Loop() {
		renderReport(in)
	}
}

func BenchmarkReadResults(b *testing.B) {
	benches := map[string][2]float64{}
	for i := range 100 {
		benches[fmt.Sprintf("Demo/case_%d", i)] = [2]float64{float64(1000 + i), float64(i % 7)}
	}
	file := writeTemp(b, "results.txt", benchOutput("github.com/grafana/alloy/internal/demo", benches))

	for b.Loop() {
		if _, err := readResults(file); err != nil {
			b.Fatal(err)
		}
	}
}
