package alloycli

/*
import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/grafana/alloy/internal/converter"
	convert_diag "github.com/grafana/alloy/internal/converter/diag"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/syntax/diag"
)

func loadFiles(path string, converterSourceFormat string, converterBypassErrors bool, configExtraArgs string) (map[string][]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		sources := map[string][]byte{}
		err := filepath.WalkDir(path, func(curPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Skip all directories and don't recurse into child dirs that aren't at top-level
			if d.IsDir() {
				if curPath != path {
					return filepath.SkipDir
				}
				return nil
			}
			// Ignore files not ending in .alloy extension
			if !strings.HasSuffix(curPath, ".alloy") {
				return nil
			}

			bb, err := os.ReadFile(curPath)
			sources[curPath] = bb
			return err
		})
		if err != nil {
			return nil, err
		}

		return sources, nil
	}

	bb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if converterSourceFormat != "alloy" {
		var diags convert_diag.Diagnostics
		ea, err := parseExtraArgs(configExtraArgs)
		if err != nil {
			return nil, err
		}

		bb, diags = converter.Convert(bb, converter.Input(converterSourceFormat), ea)
		hasError := hasErrorLevel(diags, convert_diag.SeverityLevelError)
		hasCritical := hasErrorLevel(diags, convert_diag.SeverityLevelCritical)
		if hasCritical || (!converterBypassErrors && hasError) {
			return nil, diags
		}
	}

	return map[string][]byte{path: bb}, nil
}

func printSourceErrors(source *alloy_runtime.Source) {
	var (
		diags diag.Diagnostics
		err   error
	)

	for name, e := range source.Errors() {
		// merge diagnostics for all files
		var d diag.Diagnostics
		if errors.As(e, &d) {
			diags = append(diags, d...)
			continue
		}
		err = errors.Join(err, fmt.Errorf("%s: %w", name, e))
	}

	if len(diags) > 0 {
		p := diag.NewPrinter(diag.PrinterConfig{
			Color:              !color.NoColor,
			ContextLinesBefore: 1,
			ContextLinesAfter:  1,
		})
		_ = p.Fprint(os.Stderr, source.RawConfigs(), diags)

		// Print newline after the diagnostics.
		fmt.Println()
	}

	if err != nil {
		fmt.Printf("%s\n", err)
	}
}
*/
