# Helm charts

This directory contains Helm charts for Grafana Alloy.

## Testing

The `tests` contains a list of golden templates rendered from the Helm chart.

These manifests are never run directly, but are instead used to validate the correctness of the
templates emitted by the Helm chart. To regenerate this folder, call `make generate-helm-tests` from
the root of the repository. Install Helm, Python, uv, and yamllint with
`mise install helm python uv yamllint` first.

`make generate-helm-tests` renders each `*-values.yaml` file in `charts/alloy/ci` and its
subdirectories into a separate directory under `tests`. Chart-testing also installs each top-level
`*-values.yaml` fixture. Place fixtures that require an `Always` image pull policy in
`charts/alloy/ci/render-only` so they're rendered without being installed.

When modifying the Helm charts, `make generate-helm-tests` must be run from the repo root before
submitting a PR, as a linter check will ensure that this directory is up-to-date.
