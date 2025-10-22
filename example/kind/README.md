# Alloy example using kind k8s cluster

## Dependencies

- `kind`
- `task` - taskfile.dev
- `kubectl`
- `helm`

## Setup credentials

Create a local `.env.credentials` from the provided template and fill in your values. This file is gitignored and is automatically loaded by Task via `dotenv` in `Taskfile.yml`.

```bash
cp example/kind/.env.credentials.template example/kind/.env.credentials
$EDITOR example/kind/.env.credentials
```
