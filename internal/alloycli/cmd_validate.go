package alloycli

import (
	"fmt"

	"github.com/spf13/cobra"

	alloy_runtime "github.com/grafana/alloy/internal/runtime"
)

func validateCommand() *cobra.Command {
	v := &alloyValidate{
		configFormat: "alloy",
	}

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

	// Config flags
	cmd.Flags().StringVar(&v.configFormat, "config.format", v.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	cmd.Flags().BoolVar(&v.configBypassConversionErrors, "config.bypass-conversion-errors", v.configBypassConversionErrors, "Enable bypassing errors when converting")
	cmd.Flags().StringVar(&v.configExtraArgs, "config.extra-args", v.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	return cmd
}

type alloyValidate struct {
	configFormat                 string
	configBypassConversionErrors bool
	configExtraArgs              string
}

func (v *alloyValidate) Run(configFile string) (*alloy_runtime.Source, error) {
	return loadAlloySource(configFile, v.configFormat, v.configBypassConversionErrors, v.configExtraArgs)
}
