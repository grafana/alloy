#!/usr/bin/env sh
if [ -f "{{.EnvFile}}" ]; then
  set -a
  . "{{.EnvFile}}"
  set +a
fi

extra_args=""
[ -f "{{.ExtraArgsFile}}" ] && extra_args=$(cat "{{.ExtraArgsFile}}")

exec "{{.AlloyBin}}" run \
  --storage.path="{{.StoragePath}}" \
  $extra_args \
  "{{.ConfigPath}}"
