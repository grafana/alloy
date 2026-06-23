#!/usr/bin/env sh
if [ -f "/opt/homebrew/etc/grafana-alloy/config.env" ]; then
  set -a
  # shellcheck disable=SC1091  # env file only exists at runtime
  . "/opt/homebrew/etc/grafana-alloy/config.env"
  set +a
fi

# Run in OTel mode when ALLOY_OTEL_MODE is truthy.
otel_mode=""
case "${ALLOY_OTEL_MODE:-}" in
  1 | true | yes | on ) otel_mode="1" ;;
esac

extra_args_file="/opt/homebrew/etc/grafana-alloy/extra-args.txt"
[ -n "$otel_mode" ] && extra_args_file="/opt/homebrew/etc/grafana-alloy/otel-extra-args.txt"

extra_args=""
[ -f "$extra_args_file" ] && extra_args=$(cat "$extra_args_file")

# extra_args is intentionally unquoted so a file with multiple arguments
# word-splits into separate argv entries.
if [ -n "$otel_mode" ]; then
  # shellcheck disable=SC2086
  exec "/opt/homebrew/opt/grafana-alloy/bin/alloy" otel \
    --config="file:/opt/homebrew/etc/grafana-alloy/config.yaml" \
    $extra_args
else
  # shellcheck disable=SC2086
  exec "/opt/homebrew/opt/grafana-alloy/bin/alloy" run \
    --storage.path="/opt/homebrew/var/lib/grafana-alloy/data" \
    $extra_args \
    "/opt/homebrew/etc/grafana-alloy"
fi
