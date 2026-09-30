#!/usr/bin/env bash
set -euo pipefail
operator=${OPERATOR_DIR:-$HOME/workspace/opentelemetry-operator}
arch=$(kubectl --context "kind-${CLUSTERNAME:-alloy-example}" get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')
env -u GOROOT GOMAXPROCS=2 GOMEMLIMIT=1GiB GOFLAGS='-p=2 -mod=readonly' \
  make -C "$operator" targetallocator GOOS=linux ARCH="$arch"
context=build/target-allocation/local-image
mkdir -p "$context/bin"
cp "$operator/cmd/otel-allocator/bin/targetallocator_$arch" "$context/bin/"
cp "$operator/cmd/otel-allocator/Dockerfile" "$context/"
docker build --platform "linux/$arch" --build-arg "TARGETARCH=$arch" -t target-allocator-local:demo "$context"
kind load docker-image target-allocator-local:demo --name "${CLUSTERNAME:-alloy-example}"
