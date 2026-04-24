#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CLUSTER_NAME="alloy-k8s-integration"
KUBECONFIG_PATH="${REPO_ROOT}/.tmp/integration-tests/k8s/kubeconfig"
ALLOY_IMAGE="${ALLOY_IMAGE:-grafana/alloy:latest}"
REUSE_CLUSTER=false
SKIP_ALLOY_IMAGE=false
SHARD=""
PACKAGE_SCOPE="./integration-tests/k8s/tests/..."
RUN_REGEX=""

log() {
  echo "[k8s-itest] $*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

cleanup() {
  if [[ "${REUSE_CLUSTER}" == "true" ]]; then
    log "reuse mode enabled; keeping cluster ${CLUSTER_NAME}"
    return
  fi
  log "deleting kind cluster ${CLUSTER_NAME}"
  kind delete cluster --name "${CLUSTER_NAME}" || true
}

cluster_exists() {
  kind get clusters | rg -qx "${CLUSTER_NAME}"
}

usage() {
  cat <<EOF
Usage: ./integration-tests/k8s/run.sh [flags]

Flags:
  --reuse-cluster      Reuse fixed kind cluster and keep it after test run
  --skip-alloy-image   Do not run make alloy-image (requires image to exist)
  --shard i/n          Forward shard to tests via -args -shard=i/n
  --package <path>     Run one package path (default: ./integration-tests/k8s/tests/...)
  --run <regex>        Forward -run regex to go test
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --reuse-cluster)
      REUSE_CLUSTER=true
      shift
      ;;
    --skip-alloy-image)
      SKIP_ALLOY_IMAGE=true
      shift
      ;;
    --shard)
      SHARD="${2:-}"
      shift 2
      ;;
    --package)
      PACKAGE_SCOPE="${2:-}"
      shift 2
      ;;
    --run)
      RUN_REGEX="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown flag: $1" >&2
      usage
      exit 1
      ;;
  esac
done

require_cmd docker
require_cmd kind
require_cmd kubectl
require_cmd helm
require_cmd go
require_cmd make
require_cmd rg

cd "${REPO_ROOT}"
mkdir -p "$(dirname "${KUBECONFIG_PATH}")"

if [[ "${SKIP_ALLOY_IMAGE}" == "false" ]]; then
  log "building alloy image"
  make alloy-image
else
  log "skipping alloy image build"
  docker image inspect "${ALLOY_IMAGE}" >/dev/null
fi

if cluster_exists; then
  if [[ "${REUSE_CLUSTER}" == "true" ]]; then
    log "reusing existing cluster ${CLUSTER_NAME}"
  else
    log "cluster already exists, deleting stale cluster first"
    kind delete cluster --name "${CLUSTER_NAME}"
    kind create cluster --name "${CLUSTER_NAME}"
  fi
else
  log "creating kind cluster ${CLUSTER_NAME}"
  kind create cluster --name "${CLUSTER_NAME}"
fi

trap cleanup EXIT INT TERM

kind get kubeconfig --name "${CLUSTER_NAME}" > "${KUBECONFIG_PATH}"
export KUBECONFIG="${KUBECONFIG_PATH}"
export ALLOY_K8S_MANAGED_CLUSTER=1
export ALLOY_IMAGE

log "loading required images to kind"
kind load docker-image "${ALLOY_IMAGE}" --name "${CLUSTER_NAME}"
docker build -t prom-gen:latest -f "${REPO_ROOT}/integration-tests/docker/configs/prom-gen/Dockerfile" "${REPO_ROOT}"
docker pull prom/blackbox-exporter:v0.25.0
kind load docker-image prom-gen:latest --name "${CLUSTER_NAME}"
kind load docker-image prom/blackbox-exporter:v0.25.0 --name "${CLUSTER_NAME}"

PROM_OPERATOR_VERSION="${PROM_OPERATOR_VERSION:-v0.81.0}"
log "installing prometheus operator bundle ${PROM_OPERATOR_VERSION}"
kubectl apply --server-side -f "https://github.com/prometheus-operator/prometheus-operator/releases/download/${PROM_OPERATOR_VERSION}/bundle.yaml"

GO_TEST_ARGS=(-tags="gore2regex alloyintegrationtests" -timeout 30m)
if [[ -n "${RUN_REGEX}" ]]; then
  GO_TEST_ARGS+=(-run "${RUN_REGEX}")
fi
GO_TEST_ARGS+=("${PACKAGE_SCOPE}")
if [[ -n "${SHARD}" ]]; then
  log "running shard ${SHARD}"
  GO_TEST_ARGS+=(-args -shard="${SHARD}")
fi

log "running go test ${PACKAGE_SCOPE}"
go test "${GO_TEST_ARGS[@]}"
