#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
cluster=${CLUSTERNAME:-alloy-example}
export KUBECONFIG="${KUBECONFIG:-build/kubeconfig.yaml}"
if [[ ${1:?Expected up, down or check} == down ]]; then
  "${KIND:-kind}" delete cluster --name "$cluster"
  exit
fi
active=$(docker ps --filter label=io.x-k8s.kind.cluster --format '{{.Label "io.x-k8s.kind.cluster"}}')
if [[ -n $(printf '%s\n' "$active" | grep -vx "$cluster" || true) ]]; then
  echo 'Stop other running kind clusters first.' >&2; exit 1
fi
if [[ $1 == up ]]; then
  mkdir -p build
  if ! "${KIND:-kind}" get clusters | grep -qx "$cluster"; then
    "${KIND:-kind}" create cluster --name "$cluster" --config config/target-allocation/kind.yaml --kubeconfig "$KUBECONFIG"
  else
    "${KIND:-kind}" export kubeconfig --name "$cluster" --kubeconfig "$KUBECONFIG"
  fi
fi
kubectl --context "kind-$cluster" cluster-info >/dev/null
