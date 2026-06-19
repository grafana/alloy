#!/usr/bin/env sh
if [ -f "{{.EnvFile}}" ]; then
  set -a
  . "{{.EnvFile}}"
  set +a
fi

extra_args=""
[ -f "{{.ExtraArgsFile}}" ] && extra_args=$(cat "{{.ExtraArgsFile}}")

# Run in OTel mode when ALLOY_OTEL_MODE is truthy. The `otel` subcommand has
# different flags than `run`: it takes a `--config` file URL and does not
# support `--storage.path`.
otel_mode=""
case "${ALLOY_OTEL_MODE:-}" in
  1 | true | yes | on ) otel_mode="1" ;;
esac

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
