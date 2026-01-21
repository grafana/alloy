#!/bin/bash
set -e

echo "Running benchmarks and saving to after.txt..."
go test -bench=. -benchmem -count=6 | tee after.txt

echo ""
echo "========================================="
echo "Benchstat comparison (before vs after):"
echo "========================================="
benchstat before.txt after.txt
