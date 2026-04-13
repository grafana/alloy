#!/usr/bin/env bash
# Builds the upstream OpenTelemetry opampsupervisor binary at the Collector Contrib
# version pinned in collector/go.mod 

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

GOOS="${1:?usage: install-opampsupervisor.sh GOOS GOARCH OUTPUT [GOARM]}"
GOARCH="${2:?}"
OUT="${3:?}"
GOARM="${4:-}"

ver="${OPAMP_SUPERVISOR_VERSION:-}"
if [[ -z "$ver" ]]; then
  ver="$(grep -m 1 'github.com/open-telemetry/opentelemetry-collector-contrib/' collector/go.mod | awk '{print $NF}')"
fi
if [[ -z "$ver" ]]; then
  echo "install-opampsupervisor: set OPAMP_SUPERVISOR_VERSION or ensure collector/go.mod pins opentelemetry-collector-contrib" >&2
  exit 1
fi

staging="$(mktemp -d)"
work="$(mktemp -d)"
cleanup() {
  rm -rf "$staging" "$work"
}
trap cleanup EXIT

ext=""
if [[ "$GOOS" == "windows" ]]; then
  ext=".exe"
fi

contrib="${work}/opentelemetry-collector-contrib"
mkdir -p "$contrib"
cd "$contrib"
git init -q
git remote add origin https://github.com/open-telemetry/opentelemetry-collector-contrib.git
git config core.sparseCheckout true
printf '%s\n' 'cmd/opampsupervisor' >.git/info/sparse-checkout

GIT_TERMINAL_PROMPT=0 git fetch -q --depth 1 origin "refs/tags/${ver}:refs/tags/${ver}" || {
  echo "install-opampsupervisor: could not fetch tag ${ver} from opentelemetry-collector-contrib" >&2
  exit 1
}
git checkout -q FETCH_HEAD

cd cmd/opampsupervisor

export GOOS GOARCH GOEXPERIMENT=
export CGO_ENABLED=0
export GOWORK=off
if [[ -n "$GOARM" ]]; then
  export GOARM
fi

binary="${staging}/opampsupervisor${ext}"
go build -trimpath -o "$binary" .

mkdir -p "$(dirname "$OUT")"
mv "$binary" "$OUT"
