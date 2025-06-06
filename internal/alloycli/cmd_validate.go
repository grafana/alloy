package alloycli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/otel"
	"github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/grafana/alloy/internal/service/ui"
	"github.com/grafana/alloy/internal/validator"
	"github.com/spf13/cobra"
)

func validateCommand() *cobra.Command {
	v := &alloyValidate{
		configFormat: "alloy",
		minStability: featuregate.StabilityGenerallyAvailable,
	}

	cmd := &cobra.Command{
		Use:          "validate [flags] file",
		Short:        "Validate a configuration file",
		Long:         ``,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return v.Run(args[0])
		},
	}

	// Config flags
	cmd.Flags().StringVar(&v.configFormat, "config.format", v.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	cmd.Flags().BoolVar(&v.configBypassConversionErrors, "config.bypass-conversion-errors", v.configBypassConversionErrors, "Enable bypassing errors when converting")
	cmd.Flags().StringVar(&v.configExtraArgs, "config.extra-args", v.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	cmd.Flags().Var(&v.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	cmd.Flags().BoolVar(&v.enableCommunityComps, "feature.community-components.enabled", v.enableCommunityComps, "Enable community components.")

	return cmd
}

type alloyValidate struct {
	configFormat                 string
	configBypassConversionErrors bool
	configExtraArgs              string

	minStability         featuregate.Stability
	enableCommunityComps bool
}

func (v *alloyValidate) Run(configFile string) error {
	sources, err := loadSourceFiles(configFile, v.configFormat, v.configBypassConversionErrors, v.configExtraArgs)
	if err != nil {
		return err
	}

	if err := validator.Validate(
		validator.Options{
			Sources: sources,
			ServiceDefinitions: getServiceDefinitions(
				&cluster.Service{},
				&http.Service{},
				&labelstore.Service{},
				&livedebugging.Service{},
				&otel.Service{},
				&remotecfg.Service{},
				&ui.Service{},
			),
			ComponentRegistry: component.NewDefaultRegistry(v.minStability, v.enableCommunityComps),
			MinStability:      v.minStability,
		},
	); err != nil {
		validator.Report(os.Stderr, err, sources)
		return errors.New("validation failed")
	}

	return nil
}

func getServiceDefinitions(services ...service.Service) []service.Definition {
	def := make([]service.Definition, 0, len(services))
	for _, s := range services {
		def = append(def, s.Definition())
	}
	return def
}
