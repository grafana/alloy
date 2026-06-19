# service-wrapper-gen

Generates the `alloy-wrapper` service entrypoint script used by the Grafana
Alloy Homebrew formula.

## What it does

The Homebrew formula installs a small `alloy-wrapper` shell script that its
`service` block runs as the launchd/systemd entrypoint. The wrapper sources the
environment file, reads the extra-args file, and `exec`s alloy.

The subcommand is chosen at runtime from the environment: if `ALLOY_OTEL_MODE`
is `1`, `true`, `yes`, or `on`, the wrapper runs `alloy otel`; otherwise it
runs `alloy run`. The two modes take different flags — `run` uses
`--storage.path` and the config path argument, while `otel` uses
`--config="file:<config-path>/config.yaml"` and does not support
`--storage.path`. Set the variable in the formula's `config.env` to switch a
single service between modes — Homebrew supports only one `service` block per
formula, so the mode is a runtime toggle rather than a second service.

This program emits that script. All Homebrew paths are passed as flags, and the
script body lives in `wrapper.tpl` (embedded via `//go:embed`) so it reads and
edits as a plain shell script. Keeping the generator in-tree means the wrapper
is versioned with the Alloy release it ships in, rather than maintained
separately inside the formula.

## Why a separate module

The program imports nothing outside the Go standard library, so `go run .`
needs no module downloads when invoked inside the Homebrew build sandbox. It
lives in its own module under `packaging/` — rather than under `tools/`, which
depends on Cobra and would drag every tool's dependencies into the build — to
keep it dependency-free.

## Usage

```sh
go run . \
  --alloy-bin=/opt/homebrew/opt/grafana-alloy/bin/alloy \
  --config-path=/opt/homebrew/etc/grafana-alloy \
  --storage-path=/opt/homebrew/var/lib/grafana-alloy/data \
  --env-file=/opt/homebrew/etc/grafana-alloy/config.env \
  --extra-args-file=/opt/homebrew/etc/grafana-alloy/extra-args.txt \
  --otel-extra-args-file=/opt/homebrew/etc/grafana-alloy/otel-extra-args.txt \
  --out=alloy-wrapper
```

| Flag                     | Required | Meaning                                           |
| ------------------------ | -------- | ------------------------------------------------- |
| `--alloy-bin`            | yes      | Absolute path to the `alloy` binary.              |
| `--config-path`          | yes      | Config file or directory passed to the subcommand.|
| `--storage-path`         | yes      | Value for `--storage.path` (run mode).            |
| `--env-file`             | yes      | Path to `config.env` sourced at startup.          |
| `--extra-args-file`      | yes      | Path to the run-mode extra-args file.             |
| `--otel-extra-args-file` | yes      | Path to the otel-mode extra-args file.            |
| `--out`                  | no       | Output file path; defaults to stdout.             |

Each mode reads its own extra-args file: run mode uses `--extra-args-file`,
otel mode uses `--otel-extra-args-file`.

With `--out`, the script is written with mode `0755`. Without it, the script is
written to stdout.

## Consuming from a formula

```ruby
cd "packaging/homebrew/service-wrapper-gen" do
  system "go", "run", ".",
    "--alloy-bin=#{opt_bin}/alloy",
    "--config-path=#{pkgetc}",
    "--storage-path=#{var}/lib/grafana-alloy/data",
    "--env-file=#{pkgetc}/config.env",
    "--extra-args-file=#{pkgetc}/extra-args.txt",
    "--otel-extra-args-file=#{pkgetc}/otel-extra-args.txt",
    "--out=#{buildpath}/alloy-wrapper"
end
bin.install "alloy-wrapper"
```

## Tests

`go test ./...` renders the script and compares it against the golden files
in `testdata/`. Regenerate them after an intentional template change:

```sh
UPDATE_GOLDEN=1 go test ./...
```
