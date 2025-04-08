package alloycli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	alloy_runtime "github.com/grafana/alloy/internal/runtime"
)

func validateCommand() *cobra.Command {
	v := &alloyValidate{}

	cmd := &cobra.Command{
		Use:          "validate [flags] file",
		Short:        "Validate a configuration file",
		Long:         ``,
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			source, err := v.Run(args[0])
			if err != nil {
				return fmt.Errorf("encountered errors during validation: %w", err)
			}

			diags := source.Diagnostics()
			if len(diags) > 0 {
				printDiagnostics(diags, source)
			}

			return fmt.Errorf("encountered errors during validation")
		},
	}

	return cmd
}

type alloyValidate struct{}

func (fv *alloyValidate) Run(configFile string) (*alloy_runtime.Source, error) {
	return loadSources(configFile)
}

func loadSources(path string) (*alloy_runtime.Source, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		raw := map[string][]byte{}
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
			if err != nil {
				return err
			}

			raw[curPath] = bb
			return err
		})

		if err != nil {
			return nil, err
		}

		return alloy_runtime.ParseSources2(raw), nil
	}

	bb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return alloy_runtime.ParseSources2(map[string][]byte{path: bb}), nil
}
