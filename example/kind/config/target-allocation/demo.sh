#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
root=config/target-allocation
export KUBECONFIG="${KUBECONFIG:-build/kubeconfig.yaml}"
kube=(kubectl --context "kind-${CLUSTERNAME:-alloy-example}" -n target-allocation)
action=${1:?Expected mode, alloys, extra, targets-reset, reset or profiles}
bash "$root/cluster.sh" check
mark() {
  mkdir -p build/target-allocation
  printf '%s %s\n' "$(date -u +%FT%TZ)" "$1" | tee -a build/target-allocation/events.log
  if command -v gcx >/dev/null; then
    jq -n --arg text "$1" '{dashboardUID:"target-allocation-demo",tags:["target-allocation-demo"],text:$text}' |
      gcx --context "${GCX_CONTEXT:-thampiotr}" api /api/annotations -d @- >/dev/null || echo 'Cloud annotation failed; event saved locally.' >&2
  fi
}
case "$action" in
  mode)
    mode=${2:?Expected vanilla, ta-hashing or ta-packing}
    case "$mode" in vanilla|ta-hashing|ta-packing) ;; *) echo 'Invalid mode' >&2; exit 1;; esac
    mark "Deploy $mode (transition starts)"
    REUSE_IMAGES=1 bash "$root/deploy.sh" "$mode"
    mark "$mode deployed; allow 90s to settle"
    ;;
  alloys)
    replicas=${2:?Expected replica count 3, 4 or 5}
    case "$replicas" in 3|4|5) ;; *) echo 'Demo supports 3, 4 or 5 Alloy instances' >&2; exit 1;; esac
    mark "Scale Alloy to $replicas instances"
    "${kube[@]}" scale statefulset alloy-target-allocation --replicas="$replicas"
    "${kube[@]}" rollout status statefulset/alloy-target-allocation --timeout=180s
    ;;
  extra)
    mark 'Add 8 mixed targets (+8378 synthetic series)'
    "${kube[@]}" apply -f "$root/common/targets-extra.yaml"
    for name in targets-extra-search targets-extra-worker; do "${kube[@]}" rollout status "statefulset/$name" --timeout=180s; done
    ;;
  targets-reset)
    mark 'Remove extra target batch'
    "${kube[@]}" delete -f "$root/common/targets-extra.yaml" --ignore-not-found
    ;;
  reset)
    bash "$root/demo.sh" targets-reset
    bash "$root/demo.sh" alloys 3
    "${kube[@]}" apply -f "$root/common/targets.yaml"
    ;;
  profiles)
    out="build/target-allocation/profiles/$(date -u +%Y%m%dT%H%M%SZ)"
    mkdir -p "$out"
    for pod in $("${kube[@]}" get pods -l app.kubernetes.io/name=alloy -o jsonpath='{.items[*].metadata.name}'); do
      "${kube[@]}" get --raw "/api/v1/namespaces/target-allocation/pods/$pod:12345/proxy/debug/pprof/heap" > "$out/$pod.heap.pprof"
      "${kube[@]}" get --raw "/api/v1/namespaces/target-allocation/pods/$pod:12345/proxy/debug/pprof/profile?seconds=10" > "$out/$pod.cpu.pprof"
    done
    echo "Saved local profiles in $out (go tool pprof -http=127.0.0.1:8082 FILE)"
    ;;
  *) echo "Unknown action: $action" >&2; exit 1;;
esac
