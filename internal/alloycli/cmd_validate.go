package alloycli

import (
	"fmt"

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

			if !source.HasErrors() {
				return nil
			}

			printSourceErrors(source)
			return fmt.Errorf("encountered errors during validation")
		},
	}

	return cmd
}

type alloyValidate struct{}

func (fv *alloyValidate) Run(configFile string) (*alloy_runtime.Source, error) {
	return loadAlloySource(configFile, "", false, "")
}
