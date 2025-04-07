package alloycli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/syntax/diag"

	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
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
			var err error

			if len(args) == 0 {
				// Read from stdin when there are no args provided.
				err = v.Run("-")
			} else {
				err = v.Run(args[0])
			}

			var diags diag.Diagnostics
			if errors.As(err, &diags) {
				for _, diag := range diags {
					fmt.Fprintln(os.Stderr, diag)
				}
				return fmt.Errorf("encountered errors during formatting")
			}

			return err
		},
	}

	return cmd
}

type alloyValidate struct{}

func (fv *alloyValidate) Run(configFile string) error {
	r, err := configReader(configFile)
	if err != nil {
		return err
	}
	defer r.Close()

	bb, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	source, err := alloy_runtime.ParseSource(configFile, bb)
	if err != nil {
		return err
	}

	err = alloy_runtime.TypeCheck(source, alloy_runtime.Options{
		ControllerID: "",
		Logger:       logging.NewNop(),
		DataPath:     "",
		Reg:          nil,
		MinStability: featuregate.StabilityExperimental,
		OnExportsChange: func(exports map[string]any) {
			fmt.Println("On exports change")
		},
		Services:             []service.Service{},
		EnableCommunityComps: false,
	})

	if err != nil {
		var diags diag.Diagnostics
		if errors.As(err, &diags) {
			printDiagnostics(diags, source)
			return fmt.Errorf("could not perform the initial load successfully")
		}

		// Exit if the initial load fails.
		return err
	}

	return nil
}

func configReader(configFile string) (io.ReadCloser, error) {
	switch configFile {
	case "-":
		return os.Stdin, nil
	default:
		fi, err := os.Stat(configFile)
		if err != nil {
			return nil, err
		}
		if fi.IsDir() {
			// FIXME(better error):
			return nil, fmt.Errorf("cannot format a directory")
		}

		f, err := os.Open(configFile)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
}
