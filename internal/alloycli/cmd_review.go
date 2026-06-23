package alloycli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/reviewer"
)

func reviewCommand() *cobra.Command {
	r := &alloyReview{configFormat: "alloy"}

	cmd := &cobra.Command{
		Use:          "review [flags] file",
		Short:        "",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return r.Run(args[0])
		},
	}

	// Config flags
	cmd.Flags().StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	cmd.Flags().StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")
	cmd.Flags().BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")

	return cmd
}

type alloyReview struct {
	configFormat                 string
	configBypassConversionErrors bool
	configExtraArgs              string
}

func (v *alloyReview) Run(configFile string) error {
	sources, err := loadSourceFiles(configFile, v.configFormat, v.configBypassConversionErrors, v.configExtraArgs)
	if err != nil {
		return err
	}

	res, err := reviewer.Review(reviewer.Options{
		Sources:           sources,
		ComponentRegistry: component.NewDefaultRegistry(featuregate.StabilityExperimental, true),
	})
	if err != nil {
		return err
	}

	reviewer.Report(os.Stdout, res)
	return nil
}
