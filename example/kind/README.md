# Alloy example using kind k8s cluster

## Dependencies

- `kind`
- `task` - taskfile.dev
- `kubectl`
- `helm`

## Usage

The main command to get started with is:

```bash
task help   # list all available tasks
```

Among other things, you can create a kind cluster, deploy the local Grafana Alloy helm chart, and open k9s with the local kind cluster.

```bash
task cluster:create                       # create a kind cluster
task deploy:alloy-helm                    # deploy the local Grafana Alloy helm chart
k9s --kubeconfig ./build/kubeconfig.yaml  # open k9s with the local kind cluster
```

## Setup credentials

This is required if you want to deploy the Grafana Cloud Onboarding helm chart or other features that connect to Grafana Cloud.

Create a local `.env.credentials` from the provided template and fill in your values. This file is gitignored and is automatically loaded by Task via `dotenv` in `Taskfile.yml`.

```bash
cp example/kind/.env.credentials.template example/kind/.env.credentials
$EDITOR example/kind/.env.credentials
```
