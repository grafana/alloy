package alloyservicewrapper

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type flags struct {
	alloyBin      string
	configPath    string
	storagePath   string
	envFile       string
	extraArgsFile string
	out           string
}

// Command returns the `alloy-service-wrapper` generator subcommand. It emits the
// alloy-wrapper service entrypoint script used by the Homebrew formulas, with
// all Homebrew paths supplied as flags so a single template serves both the
// homebrew-core and homebrew-grafana formulas.
func Command() *cobra.Command {
	var f flags

	cmd := &cobra.Command{
		Use:   "alloy-service-wrapper",
		Short: "Generate the alloy-wrapper service entrypoint script for Homebrew formulas",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, f)
		},
	}

	cmd.Flags().StringVar(&f.alloyBin, "alloy-bin", "", "Absolute path to the alloy binary")
	cmd.Flags().StringVar(&f.configPath, "config-path", "", "Config file or directory passed to `alloy run`")
	cmd.Flags().StringVar(&f.storagePath, "storage-path", "", "Value for --storage.path")
	cmd.Flags().StringVar(&f.envFile, "env-file", "", "Path to the environment file sourced at startup")
	cmd.Flags().StringVar(&f.extraArgsFile, "extra-args-file", "", "Path to the extra-args file")
	cmd.Flags().StringVar(&f.out, "out", "", "Output file path (default: stdout)")

	for _, name := range []string{"alloy-bin", "config-path", "storage-path", "env-file", "extra-args-file"} {
		_ = cmd.MarkFlagRequired(name)
	}

	return cmd
}

func run(cmd *cobra.Command, f flags) error {
	data := templateData{
		AlloyBin:      f.alloyBin,
		ConfigPath:    f.configPath,
		StoragePath:   f.storagePath,
		EnvFile:       f.envFile,
		ExtraArgsFile: f.extraArgsFile,
	}

	var buf bytes.Buffer
	if err := render(&buf, data); err != nil {
		return err
	}

	if f.out == "" {
		_, err := cmd.OutOrStdout().Write(buf.Bytes())
		return err
	}

	if err := os.WriteFile(f.out, buf.Bytes(), 0o755); err != nil {
		return fmt.Errorf("writing wrapper to %s: %w", f.out, err)
	}
	return nil
}
