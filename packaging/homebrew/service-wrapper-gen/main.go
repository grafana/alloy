// Command service-wrapper-gen generates the `alloy-wrapper` service entrypoint
// script consumed by the Grafana Alloy Homebrew formulas (homebrew-core and the
// homebrew-grafana tap).
//
// All Homebrew paths are supplied as flags so a single template serves both
// formulas; the program deliberately depends only on the Go standard library so
// `go run` needs no module downloads inside the Homebrew build sandbox.
package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"
)

// wrapperTemplate is the service wrapper script template. It lives in its own
// file so it can be read and edited as a plain shell script.
//
//go:embed wrapper.tpl
var wrapperTemplate string

// templateData holds the values interpolated into wrapperTemplate. All fields
// are absolute paths supplied by the consuming Homebrew formula.
type templateData struct {
	// AlloyBin is the absolute path to the alloy binary.
	AlloyBin string
	// ConfigPath is the config file or directory passed to `alloy run`.
	ConfigPath string
	// StoragePath is the value for --storage.path.
	StoragePath string
	// EnvFile is the environment file sourced at startup.
	EnvFile string
	// ExtraArgsFile holds extra command line arguments passed to `alloy run`.
	ExtraArgsFile string
	// OtelExtraArgsFile holds extra command line arguments passed to
	// `alloy otel` (OTel mode uses a separate file from run mode).
	OtelExtraArgsFile string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("service-wrapper-gen", flag.ContinueOnError)

	var (
		data templateData
		out  string
	)
	fs.StringVar(&data.AlloyBin, "alloy-bin", "", "Absolute path to the alloy binary (required)")
	fs.StringVar(&data.ConfigPath, "config-path", "", "Config file or directory passed to `alloy run` (required)")
	fs.StringVar(&data.StoragePath, "storage-path", "", "Value for --storage.path (required)")
	fs.StringVar(&data.EnvFile, "env-file", "", "Path to the environment file sourced at startup (required)")
	fs.StringVar(&data.ExtraArgsFile, "extra-args-file", "", "Path to the run-mode extra-args file (required)")
	fs.StringVar(&data.OtelExtraArgsFile, "otel-extra-args-file", "", "Path to the otel-mode extra-args file (required)")
	fs.StringVar(&out, "out", "", "Output file path (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	rendered, err := render(data)
	if err != nil {
		return err
	}

	if out == "" {
		_, err = os.Stdout.Write(rendered)
		return err
	}
	if err := os.WriteFile(out, rendered, 0o755); err != nil {
		return fmt.Errorf("writing wrapper to %s: %w", out, err)
	}
	return nil
}

// validate ensures every field is set and free of characters that would break
// the generated shell script (the values are interpolated inside double quotes).
func (d templateData) validate() error {
	fields := []struct {
		flag, value string
	}{
		{"alloy-bin", d.AlloyBin},
		{"config-path", d.ConfigPath},
		{"storage-path", d.StoragePath},
		{"env-file", d.EnvFile},
		{"extra-args-file", d.ExtraArgsFile},
		{"otel-extra-args-file", d.OtelExtraArgsFile},
	}
	for _, f := range fields {
		if f.value == "" {
			return fmt.Errorf("flag --%s is required", f.flag)
		}
		if strings.ContainsAny(f.value, "\"\n") {
			return fmt.Errorf("flag --%s contains invalid characters (quote or newline): %q", f.flag, f.value)
		}
	}
	return nil
}

// render returns the rendered wrapper script.
func render(data templateData) ([]byte, error) {
	if err := data.validate(); err != nil {
		return nil, err
	}

	tmpl, err := template.New("wrapper").Parse(wrapperTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing wrapper template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering wrapper template: %w", err)
	}
	return buf.Bytes(), nil
}
