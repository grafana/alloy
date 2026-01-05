
# generate-module-dependencies

A small utility to keep Go module replace directives consistent across the repository from a single source of truth.

## What it does

- Reads dependency definitions from the project-level `dependency-replacements.yaml`.
- Renders the list of `replace` directives using a template.
- Injects the rendered block into target files (e.g., `go.mod` or OCB builder config YAML files) between well-known markers.
- Runs `go mod tidy` for affected modules with `file_type` of mod

Generated blocks are wrapped with:
```
BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY ...  END GENERATED REPLACES
```

**Do not edit anything between these markers**, manually—update `dependency-replacements.yaml` instead. Anything 
changed manually within these markers will be overwritten during the next run.

Please note that local replacement directives (ie pointing a dependency to a local module) are _not_ meant to be included
in `dependency-replacements.yaml`. These must be included separately in `go.mod` files, outside of the template boundaries.

## Supported File Types

The tool supports two file types:

- **`mod`**: Go module files (`go.mod`)
- **`ocb`**: OpenTelemetry Collector Builder (OCB) config YAML files

## Usage

- One-off run from repo root:
    - `make generate-module-dependencies`
    - or `cd tools/generate-module-dependencies && go generate`

## Configuration

All inputs come from `dependency-replacements.yaml`, which defines:
- Modules to update (name, path, file_type).
  - `file_type` can be `mod` for `go.mod` files or `ocb` for OCB builder config YAML files
- Replace entries (dependency, replacement, optional comment).

Comments are normalized (single-line) and included above the corresponding `replace` directive in generated output.

### Example `dependency-replacements.yaml`

```yaml
modules:
  - name: main
    path: go.mod
    file_type: mod
  - name: collector
    path: collector/builder-config.yaml
    file_type: ocb

replaces:
  - dependency: example.com/package
    replacement: example.com/fork v1.0.0
    comment: Test replace for example.com/package
  - dependency: github.com/test/dependency
    replacement: github.com/test/fork v1.0.0
    comment: Another test replace
```

## Troubleshooting

- If a start marker exists without an end marker (or vice versa), generation fails—ensure both markers are present or absent together.
- If `go mod tidy` fails, fix the underlying module issues and rerun the command above.

## Notes

- This tool writes only the generated block; existing content outside the markers is preserved.
- The CI will run a check and fail if the output of the generate-module-dependencies step differs from the committed version.
