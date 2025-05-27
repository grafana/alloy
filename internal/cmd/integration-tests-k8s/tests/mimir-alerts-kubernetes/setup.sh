#!/bin/bash
set -x #echo on

# Install the Prometheus Operator
LATEST=$(curl -s https://api.github.com/repos/prometheus-operator/prometheus-operator/releases/latest | jq -cr .tag_name)
curl -sL https://github.com/prometheus-operator/prometheus-operator/releases/download/${LATEST}/bundle.yaml | kubectl create -f -

# Install Mimir
kubectl create namespace mimir-test
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
helm -n mimir-test install mimir grafana/mimir-distributed
