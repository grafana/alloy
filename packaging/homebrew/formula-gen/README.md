# formula-gen

Renders the Grafana Alloy Homebrew formula template (`../alloy.rb.tpl`) into a
concrete `alloy.rb` for the [homebrew-grafana](https://github.com/grafana/homebrew-grafana)
tap. Used by `.github/workflows/bump-formula-pr.yml` after a release is published.

The tool depends only on the Go standard library.

## Usage

```
go run -C packaging/homebrew/formula-gen/ . \
  -tag v1.17.1 \
  -artifacts ../artifacts.json \
  -out alloy.rb
```

### Flags

| Flag         | Required | Default          | Description                                            |
|--------------|----------|------------------|--------------------------------------------------------|
| `-tag`       | yes      |                  | Raw git tag, e.g. `v1.17.1`. `.Version` is this minus `v`. |
| `-artifacts` | no       | `../artifacts.json` | Path to the artifacts JSON file.                     |
| `-template`  | no       | `../alloy.rb.tpl`| Path to the formula template.                          |
| `-out`       | no       | `alloy.rb`       | Output file path.                                      |

### Environment variables

| Variable            | Required | Description                                                        |
|---------------------|----------|--------------------------------------------------------------------|
| `GITHUB_SERVER_URL` | yes      | e.g. `https://github.com`. Combined with the repository into `.BaseURL`. |
| `GITHUB_REPOSITORY` | yes      | e.g. `grafana/alloy`.                                              |

`.BaseURL` is built as `${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/releases/download`.

The `SHA256SUMS` asset is fetched anonymously: grafana/alloy is public and the
bump workflow only runs on published (non-draft) releases.

## Checksums

Checksums are read from the release's `SHA256SUMS` asset (produced by
`scripts/publish-release-artifacts.sh`), fetched from
`${BaseURL}/${tag}/SHA256SUMS`. The tool does not download the (large) release
archives. It fails if `SHA256SUMS` is missing an entry for any artifact
`package` listed in the artifacts JSON.

## Artifacts JSON

Maps OS/arch to the release archive (`package`) and the binary file inside it
(`binFile`):

```json
{
  "darwin": {
    "arm64": { "package": "alloy-darwin-arm64.zip", "binFile": "alloy-darwin-arm64" },
    "amd64": { "package": "alloy-darwin-amd64.zip", "binFile": "alloy-darwin-amd64" }
  },
  "linux": {
    "arm64": { "package": "alloy-linux-arm64.zip", "binFile": "alloy-linux-arm64" },
    "amd64": { "package": "alloy-linux-amd64.zip", "binFile": "alloy-linux-amd64" }
  }
}
```
