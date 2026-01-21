#!/bin/bash
set -e

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    rm -f mutex.out labelstore.test
    echo "Done."
}

# Set trap to cleanup on exit
trap cleanup EXIT

echo "Running benchmark with mutex profiling..."
go test -bench=BenchmarkHighContention -benchtime=2s -mutexprofile=mutex.out -mutexprofilefraction=1

echo ""
echo "========================================="
echo "Top Mutex Contention Points:"
echo "========================================="
go tool pprof -top mutex.out | head -20

echo ""
echo "========================================="
echo "Opening web UI at http://localhost:8080"
echo "Press Ctrl+C to stop the server and cleanup"
echo "========================================="
go tool pprof -http=:8080 mutex.out
