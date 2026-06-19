#!/usr/bin/env sh
if [ -f "{{.EnvFile}}" ]; then
  set -a
  . "{{.EnvFile}}"
  set +a
fi

# Run in OTel mode when ALLOY_OTEL_MODE is truthy.
otel_mode=""
case "${ALLOY_OTEL_MODE:-}" in
  1 | true | yes | on ) otel_mode="1" ;;
esac

extra_args_file="{{.ExtraArgsFile}}"
[ -n "$otel_mode" ] && extra_args_file="{{.OtelExtraArgsFile}}"

extra_args=""
[ -f "$extra_args_file" ] && extra_args=$(cat "$extra_args_file")

if [ -n "$otel_mode" ]; then
  exec "{{.AlloyBin}}" otel \
    --config="file:{{.ConfigPath}}/config.yaml" \
    $extra_args
else
  exec "{{.AlloyBin}}" run \
    --storage.path="{{.StoragePath}}" \
    $extra_args \
    "{{.ConfigPath}}"
fi
