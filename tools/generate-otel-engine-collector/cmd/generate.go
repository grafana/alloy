package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grafana/alloy/tools/generate-otel-engine-collector/internal/generator"
	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	var (
		collectorDir    string
		builderVersion string
		fromScratch    bool
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Run OTel builder and post-process generated collector code",
		RunE: func(cmd *cobra.Command, args []string) error {
			absDir, err := filepath.Abs(collectorDir)
			if err != nil {
				return err
			}
			collectorDir = absDir
			if builderVersion == "" {
				builderVersion = os.Getenv("BUILDER_VERSION")
			}
			if builderVersion == "" {
				return fmt.Errorf("builder version is required: set --builder-version or BUILDER_VERSION")
			}
			return generator.Generate(collectorDir, builderVersion, fromScratch)
		},
	}
	cmd.Flags().StringVar(&collectorDir, "collector-dir", "", "Path to the collector directory (contains builder-config.yaml)")
	cmd.Flags().StringVar(&builderVersion, "builder-version", "", "OTel builder version (e.g. v0.139.0); defaults to BUILDER_VERSION env")
	cmd.Flags().BoolVar(&fromScratch, "from-scratch", false, "Remove main*.go, components.go, go.mod, go.sum before generating")
	_ = cmd.MarkFlagRequired("collector-dir")
	return cmd
}
