#!/bin/bash
set -e

echo "========================================="
echo "Running benchmark with mutex and block profiling..."
echo "========================================="
echo ""

# Enable mutex profiling with rate=1 (capture all mutex events)
# Run the benchmark with profiling enabled
go test -bench=BenchmarkHighContentionMixed -benchtime=1s -mutexprofile=mutex.pprof -blockprofile=block.pprof -mutexprofilefraction=1 -blockprofilerate=1

echo ""
echo "========================================="
echo "Mutex Contention Analysis (Top 15):"
echo "========================================="
go tool pprof -top -nodefraction=0 mutex.pprof | head -25

echo ""
echo "========================================="
echo "Block Profile Analysis (Top 15):"
echo "========================================="
go tool pprof -top -nodefraction=0 block.pprof | head -25

echo ""
echo "========================================="
echo "Detailed view of mutex contention in labelstore:"
echo "========================================="
go tool pprof -list="labelstore" mutex.pprof 2>/dev/null | head -50 || echo "Run 'go tool pprof -list=labelstore mutex.pprof' for detailed source view"

echo ""
echo "========================================="
echo "Summary:"
echo "========================================="
echo "✓ Mutex profile saved to: mutex.pprof"
echo "✓ Block profile saved to: block.pprof"
echo ""
echo "To see which line of code has the most contention:"
echo "  go tool pprof -list='GetOrAddGlobalRefID' mutex.pprof"
echo "  go tool pprof -list='GetLocalRefID' mutex.pprof"
echo "  go tool pprof -list='AddLocalLink' mutex.pprof"
echo ""
echo "To interactively explore:"
echo "  go tool pprof mutex.pprof"
echo "    (then type 'top', 'list Service', or 'web')"
echo ""
echo "To generate a visual graph (requires graphviz):"
echo "  go tool pprof -web mutex.pprof"
