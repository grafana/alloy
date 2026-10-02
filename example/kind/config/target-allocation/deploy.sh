#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
mode=${1:?Expected vanilla, ta-hashing or ta-packing}
case "$mode" in vanilla|ta-hashing|ta-packing) ;; *) exit 1 ;; esac
root=config/target-allocation
export KUBECONFIG="${KUBECONFIG:-build/kubeconfig.yaml}"
kube=(kubectl --context "kind-${CLUSTERNAME:-alloy-example}")
for key in GRAFANA_CLOUD_PROMETHEUS_URL GRAFANA_CLOUD_PROMETHEUS_USERNAME GRAFANA_CLOUD_METRICS_WRITE_TOKEN; do
  test -n "${!key:-}" || { echo "Set $key in .env.credentials" >&2; exit 1; }
done
"${kube[@]}" apply -f "$root/common/namespace.yaml"
# Credentials travel through stdin, never command arguments or checked-in files.
"${kube[@]}" -n target-allocation create secret generic grafana-cloud-metrics \
  --from-env-file=<(printf '%s\n' "GRAFANA_CLOUD_PROMETHEUS_URL=$GRAFANA_CLOUD_PROMETHEUS_URL" \
    "GRAFANA_CLOUD_PROMETHEUS_USERNAME=$GRAFANA_CLOUD_PROMETHEUS_USERNAME" \
    "GRAFANA_CLOUD_METRICS_WRITE_TOKEN=$GRAFANA_CLOUD_METRICS_WRITE_TOKEN") \
  --dry-run=client -o yaml | "${kube[@]}" apply --server-side --field-manager=target-allocation-demo -f -
"${kube[@]}" apply -f "$root/common/targets.yaml" -f "$root/common/kube-state-metrics.yaml"
# Replace the retired synthetic KSM target with the real exporter.
"${kube[@]}" -n target-allocation delete statefulset targets-ksm --ignore-not-found
"${kube[@]}" -n target-allocation rollout status deployment/kube-state-metrics --timeout=180s
config="$root/vanilla/config.alloy"
if [[ $mode == ta-* ]]; then
  if [[ ${REUSE_IMAGES:-0} != 1 ]]; then bash "$root/build-ta.sh"; fi
  "${kube[@]}" apply -f "$root/$mode/allocator.yaml" -f "$root/common/allocator.yaml"
  "${kube[@]}" -n target-allocation rollout restart deployment/target-allocator
  "${kube[@]}" -n target-allocation rollout status deployment/target-allocator --timeout=180s
  config="$root/common/ta.alloy"
fi
mkdir -p build/target-allocation
cat "$config" "$root/common/metrics.alloy" | sed "s/sys.env(\"DEMO_RUN_ID\")/\"$mode\"/" > build/target-allocation/config.alloy
if [[ ${REUSE_IMAGES:-0} == 1 && $mode == ta-* ]] &&
  "${kube[@]}" -n target-allocation get configmap alloy-target-allocation -o 'jsonpath={.data.config\.alloy}' 2>/dev/null | grep -q 'discovery.http "demo"'; then
  "${kube[@]}" -n target-allocation create configmap alloy-target-allocation \
    --from-file=config.alloy=build/target-allocation/config.alloy --dry-run=client -o json | \
    jq '{data}' | "${kube[@]}" -n target-allocation patch configmap alloy-target-allocation --type merge --patch-file /dev/stdin
  printf '%s\n' "$mode" > build/target-allocation/mode
  echo 'Alloy config will hot-reload; Alloy pods retained. Allow 90s for metrics to settle.'
  exit
fi
helm --kubeconfig "$KUBECONFIG" --kube-context "kind-${CLUSTERNAME:-alloy-example}" upgrade --install \
  alloy-target-allocation ../../operations/helm/charts/alloy -n target-allocation \
  -f "$root/common/alloy-values.yaml" -f "$root/$mode/alloy-values.yaml" \
  --set-file alloy.configMap.content=build/target-allocation/config.alloy \
  --wait --timeout=5m
if [[ $mode == vanilla ]]; then
  "${kube[@]}" delete -f "$root/common/allocator.yaml" -f "$root/ta-hashing/allocator.yaml" --ignore-not-found
fi

printf '%s\n' "$mode" > build/target-allocation/mode
