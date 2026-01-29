#!/bin/bash
set -x #echo on

# Build and load the prom-gen image
# Navigate to the repo root (5 levels up from testdata/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../../.." && pwd)"

docker build -t prom-gen:latest -f "${REPO_ROOT}/integration-tests/docker/configs/prom-gen/Dockerfile" "${REPO_ROOT}"
minikube image load prom-gen:latest -p alloy-int-test-prometheus-operator

# Load the blackbox exporter image for Probe tests
docker pull prom/blackbox-exporter:v0.25.0
minikube image load prom/blackbox-exporter:v0.25.0 -p alloy-int-test-prometheus-operator

# Install the Prometheus Operator
LATEST=$(curl -s https://api.github.com/repos/prometheus-operator/prometheus-operator/releases/latest | jq -cr .tag_name)
curl -sL https://github.com/prometheus-operator/prometheus-operator/releases/download/"${LATEST}"/bundle.yaml | kubectl create -f -

# Install Mimir
kubectl create namespace mimir-test
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
# TODO: Upgrade to version 6 of Mimir's Helm chart.
helm -n mimir-test install mimir grafana/mimir-distributed --version 5.8.0 --wait
