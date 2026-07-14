#!/usr/bin/env bash
# Build the Alloy mixin and push its dashboards into a personal Grafana folder
# via gcx, using the legacy API (works with folder-scoped edit rights — no
# app-platform RBAC or org Editor role needed).
#
# Idempotent: dashboards get a stable, namespaced uid ("$PREFIX$origuid"), so
# they're your own copies (not moves of any org-wide originals) and re-running
# updates them in place. Re-run after any libsonnet change to refresh Grafana.
#
# Usage:
#   ./gcx-sync.sh                              # build + push all mixin dashboards
#   FOLDER_TITLE='@paulin.todev' ./gcx-sync.sh # target a different folder
#   SKIP_BUILD=1 ./gcx-sync.sh                 # push already-rendered JSON
#
# Requires: gcx logged in (`gcx login`) to the target stack.
set -euo pipefail

FOLDER_TITLE="${FOLDER_TITLE:-@paulin.todev}"
PARENT_UID="${PARENT_UID:-users}"   # nested-folders parent ("» Users"); "" for top-level
PREFIX="${PREFIX:-ptd-}"            # uid namespace for your personal copies
REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RENDERED="$REPO_ROOT/operations/alloy-mixin/rendered/dashboards"

# strip gcx hint lines and surrounding quotes from --jq string output
clean() { grep -v '"class":"hint"' | tr -d '"'; }

if [ "${SKIP_BUILD:-0}" != "1" ]; then
  echo "==> regenerating rendered mixin"
  make -C "$REPO_ROOT" generate-rendered-mixin
fi

echo "==> ensuring folder '$FOLDER_TITLE' exists"
uid="$(gcx api "/api/folders?parentUid=$PARENT_UID" --jq '.[] | select(.title=="'"$FOLDER_TITLE"'") | .uid' 2>/dev/null | clean | head -1 || true)"
if [ -z "$uid" ]; then
  body="$(python3 -c "import json,os;print(json.dumps({'title':os.environ['FOLDER_TITLE'],**({'parentUid':os.environ['PARENT_UID']} if os.environ.get('PARENT_UID') else {})}))" FOLDER_TITLE="$FOLDER_TITLE" PARENT_UID="$PARENT_UID")"
  uid="$(gcx api /api/folders -X POST -d "$body" --jq '.uid' | clean | head -1)"
  echo "    created folder uid=$uid"
else
  echo "    found folder uid=$uid"
fi

echo "==> importing dashboards into '$FOLDER_TITLE'"
for f in "$RENDERED"/*.json; do
  python3 -c "
import json,os
d=json.load(open('$f')); d.pop('id',None)
d['uid']=('${PREFIX}'+d.get('uid',''))[:40]
json.dump({'dashboard':d,'folderUid':'$uid','overwrite':True}, open('/tmp/gcx-dash.json','w'))"
  title="$(python3 -c "import json;print(json.load(open('$f')).get('title','?'))")"
  res="$(gcx api /api/dashboards/db -X POST -d @/tmp/gcx-dash.json --jq '.status + "  " + .url' 2>&1 | clean | head -1)"
  printf '    %-38s %s\n' "$title" "$res"
done
echo "==> done — open Grafana under the '$FOLDER_TITLE' folder."
