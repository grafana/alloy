# Using the PR-Based Dev Image Workflow

## Workflow Overview

When a pull request is labeled with any of the following:

- `publish-dev:linux`
- `publish-dev:linux-bc`
- `publish-dev:windows`

A GitHub Actions job triggers to build and push a alloy dev Docker image with a specific tag format to `/grafana/alloy-dev` registry.

## Image Tagging Convention

The image is tagged using this format:

```
pr-<pr_number>-<version>-devel+<commit_sha>
```

- `<pr_number>`: the PR number that triggered the build  
- `<version>`: the Alloy version inferred from the code  
- `<commit_sha>`: the Git SHA of the commit included in the image

**Example tag:**

```
pr-4351-1.10.0-devel+abc123def456
```

## Typical Usage Flow

1. Submit a pull request (e.g. `#4351`) for feature or bug fix.
2. Add the label `publish-dev:linux` (or `publish-dev:windows`, etc.) via the GitHub UI.
3. Monitor the GitHub Actions workflow:
   - It will build a Docker image from the PR’s code.
   - It tags the image following the format above.
   - The image is pushed to the dev registry.
