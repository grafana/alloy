package alloycli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/grafana/alloy/internal/converter"
	convert_diag "github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/syntax/diag"
)

func convertCommand() *cobra.Command {
	f := &alloyConvert{
		output:       "",
		sourceFormat: "",
		bypassErrors: false,
		extraArgs:    "",
	}

	cmd := &cobra.Command{
		Use:   "convert [flags] [file]",
		Short: "Convert a supported config file to Alloy",
		Long: `The convert subcommand translates a supported config file to
an Alloy configuration file.

If the file argument is not supplied or if the file argument is "-", then
convert will read from stdin.

The -o flag can be used to write the formatted file back to disk. When -o
is not provided, convert will write the result to stdout.

The -r flag can be used to generate a diagnostic report. When -r is not
provided, no report is generated.

The -f flag can be used to specify the format we are converting from.

The -b flag can be used to bypass errors. Errors are defined as 
non-critical issues identified during the conversion where an
output can still be generated.

The -e flag can be used to pass extra arguments to the converter
which were used by the original format. Multiple arguments can be passed
by separating them with a space.`,
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,

		RunE: func(_ *cobra.Command, args []string) error {
			var err error

			if len(args) == 0 {
				// Read from stdin when there are no args provided.
				err = f.Run("-")
			} else {
				err = f.Run(args[0])
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

	cmd.Flags().StringVarP(&f.output, "output", "o", f.output, "The filepath and filename where the output is written.")
	cmd.Flags().StringVarP(&f.report, "report", "r", f.report, "The filepath and filename where the report is written.")
	cmd.Flags().StringVarP(&f.sourceFormat, "source-format", "f", f.sourceFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	cmd.Flags().BoolVarP(&f.bypassErrors, "bypass-errors", "b", f.bypassErrors, "Enable bypassing errors when converting")
	cmd.Flags().StringVarP(&f.extraArgs, "extra-args", "e", f.extraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")
	return cmd
}

type alloyConvert struct {
	output       string
	report       string
	sourceFormat string
	bypassErrors bool
	extraArgs    string
}

func (fc *alloyConvert) Run(configFile string) error {
	if fc.sourceFormat == "" {
		return fmt.Errorf("source-format is a required flag")
	}

	if configFile == "-" {
		return convert(os.Stdin, fc)
	}

	fi, err := os.Stat(configFile)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("cannot convert a directory")
	}

	f, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return convert(f, fc)
}

func convert(r io.Reader, fc *alloyConvert) error {
	inputBytes, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	ea, err := parseExtraArgs(fc.extraArgs)
	if err != nil {
		return err
	}

	alloyBytes, diags := converter.Convert(inputBytes, converter.Input(fc.sourceFormat), ea)
	err = generateConvertReport(diags, fc)
	if err != nil {
		return err
	}

	hasError := hasErrorLevel(diags, convert_diag.SeverityLevelError)
	hasCritical := hasErrorLevel(diags, convert_diag.SeverityLevelCritical)
	if hasCritical || (!fc.bypassErrors && hasError) {
		return diags
	}

	var buf bytes.Buffer
	buf.WriteString(string(alloyBytes))

	if fc.output == "" {
		_, err := io.Copy(os.Stdout, &buf)
		return err
	}

	wf, err := os.Create(fc.output)
	if err != nil {
		return err
	}
	defer wf.Close()

	_, err = io.Copy(wf, &buf)
	return err
}

func generateConvertReport(diags convert_diag.Diagnostics, fc *alloyConvert) error {
	if fc.report != "" {
		file, err := os.Create(fc.report)
		if err != nil {
			return err
		}
		defer file.Close()

		return diags.GenerateReport(file, convert_diag.Text, fc.bypassErrors)
	}

	return nil
}

// HasErrorLevel returns true if any diagnostic exists at the provided severity.
func hasErrorLevel(ds convert_diag.Diagnostics, sev convert_diag.Severity) bool {
	for _, diag := range ds {
		if diag.Severity == sev {
			return true
		}
	}

	return false
}

func supportedFormatsList() string {
	var ret = make([]string, 0, len(converter.SupportedFormats)+1)
	ret = append(ret, "alloy")

	for _, f := range converter.SupportedFormats {
		ret = append(ret, fmt.Sprintf("%q", f))
	}
	return strings.Join(ret, ", ")
}

func parseExtraArgs(extraArgs string) ([]string, error) {
	var result []string
	if extraArgs == "" {
		return result, nil
	}

	arguments := strings.Fields(extraArgs)
	for i, arg := range arguments {
		fs := pflag.NewFlagSet("extra-args", pflag.ExitOnError)
		fs.ParseErrorsAllowlist.UnknownFlags = true
		keyStartIndex := 0
		doParseFlagValue := false

		// Split the argument into key and value.
		splitArgs := strings.SplitN(arg, "=", 2)

		// Append the key to the result.
		result = append(result, splitArgs[0])

		// If the flag has a value, add it to the FlagSet for parsing.
		if len(splitArgs) == 2 && splitArgs[1] != "" {
			doParseFlagValue = true
			if arg[1] == '-' { // longhand flag, ie. --flag=value
				keyStartIndex = 2
			} else if arg[0] == '-' { // shorthand flag, ie. -f=value
				keyStartIndex = 1
			} else { // invalid flag that was split on '=' but has no dashes in the key
				return nil, fmt.Errorf("invalid flag found: %s", arg)
			}
		}

		if doParseFlagValue {
			result = append(result, "")
			lastIndex := len(result) - 1
			key := splitArgs[0][keyStartIndex:]
			if keyStartIndex == 2 {
				fs.StringVar(&result[lastIndex], key, result[lastIndex], "")
			} else {
				// static mode uses keys with a single dash. We need to sanitize them here.
				if len(key) != 1 {
					arguments[i] = "-" + arguments[i]
					fs.StringVar(&result[lastIndex], key, result[lastIndex], "")
				} else {
					fs.StringVarP(&result[lastIndex], "", key, result[lastIndex], "")
				}
			}

			// We must parse the flag here because the pointer to the array element
			// &result[lastIndex] is overridden by the next iteration of the loop.
			// This can be improved if we preallocate the array, however we won't
			// know the final length without analyzing the arguments so there
			// is some complexity in doing so.
			err := fs.Parse(arguments)
			if err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
