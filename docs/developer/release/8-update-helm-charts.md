# Update Helm Charts

Our Helm charts require some version updates as well.

## Before you begin

1. Install [helm-docs](https://github.com/norwoodj/helm-docs) on macOS/Linux.

2. Install [yamllint](https://github.com/adrienverge/yamllint) on macOS/Linux.

## Steps

1. Create a branch from `main` for [grafana/helm-charts](https://github.com/grafana/helm-charts).

2. Update the code:

   1. Copy the content of the last CRDs into helm-charts.

      Copy the contents from agent repo `operations/agent-static-operator/crds` to replace the contents of helm-charts repo `charts/agent-operator/crds`

   2. Update references of agent-operator app version in helm-charts pointing to release version.

   3. Bump up the helm chart version.

   > **NOTE**: Do not update the README.md manually. Running the
   > [helm-docs](https://github.com/norwoodj/helm-docs) utility in the `charts/agent-operator`
   > directory will update it automatically.

3. Open a PR, following the pattern in PR [#2233](https://github.com/grafana/helm-charts/pull/2233).

4. Create a branch from `main` for [grafana/agent](https://github.com/grafana/agent).

5. Update the helm chart code in `$agentRepo/operations/helm`:

   1. Update `Chart.yaml` with the new helm version and app version.
   2. Update `CHANGELOG.md` with a new section for the helm version.
   3. Run `make docs rebuild-tests` from the `operations/helm` directory.

6. Open a PR, following the pattern in PR [#3126](https://github.com/grafana/agent/pull/3126).
