package alloyservicewrapper

import (
	_ "embed"
	"fmt"
	"io"
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

// render writes the rendered wrapper script to w.
func render(w io.Writer, data templateData) error {
	if err := data.validate(); err != nil {
		return err
	}

	tmpl, err := template.New("wrapper").Parse(wrapperTemplate)
	if err != nil {
		return fmt.Errorf("parsing wrapper template: %w", err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("rendering wrapper template: %w", err)
	}
	return nil
}
