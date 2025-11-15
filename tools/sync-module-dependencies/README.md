
# sync-module-dependencies

A small utility to keep Go module replace directives consistent across the repository from a single source of truth.

## What it does

- Reads dependency definitions from the project-level `dependency-replacements.yaml`.
- Renders the list of `replace` directives using a template.
- Injects the rendered block into target files (e.g., `go.mod`) between well-known markers.
- Runs `go mod tidy` for affected modules.
- Cleans up temporary files created during generation.

<img src="resources/flow_diagram.png" alt="flow diagram" width="200">

Generated blocks are wrapped with:
```
text BEGIN GENERATED REPLACES - DO NOT EDIT ...  END GENERATED REPLACES
```

**Do not edit anything between these markers**, manually—update `dependency-replacements.yaml` instead. Anything 
changed manually within these markers will be overwritten during the next run.

## Usage

- One-off run from repo root:
    - `make sync-module-dependencies`
    - or `cd tools/sync-module-dependencies && go generate`

- Automatically invoked by:
    - `make alloy` (locally, via a prerequisite)

## Configuration

All inputs come from `dependency-replacements.yaml`, which defines:
- Modules to update (name, path, file_type).
- Replace entries (dependency, replacement, optional comment).

Comments are normalized (single-line) and included above the corresponding `replace` directive in generated output.

## Troubleshooting

- If a start marker exists without an end marker (or vice versa), generation fails—ensure both markers are present or absent together.
- If `go mod tidy` fails, fix the underlying module issues and rerun the command above.

## Notes

- This tool writes only the generated block; existing content outside the markers is preserved.
- CI skips this step by design—commit any changes produced locally to keep the repo in sync.
