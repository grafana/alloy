package flowmode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/grafana/river/diag"
	"github.com/grafana/river/parser"
	"github.com/grafana/river/printer"
)

func fmtCommand() *cobra.Command {
	f := &flowFmt{
		write: false,
	}

	cmd := &cobra.Command{
		Use:   "fmt [flags] file",
		Short: "Format a River file",
		Long: `The fmt subcommand applies standard formatting rules to the specified
River configuration file.

If the file argument is not supplied or if the file argument is "-", then fmt will read from stdin.

The -w flag can be used to write the formatted file back to disk. -w can not be provided when fmt is reading from stdin. When -w is not provided, fmt will write the result to stdout.`,
		Args:         cobra.RangeArgs(0, 1),
		SilenceUsage: true,
		Aliases:      []string{"format"},

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

	cmd.Flags().BoolVarP(&f.write, "write", "w", f.write, "write result to (source) file instead of stdout")
	return cmd
}

type flowFmt struct {
	write bool
}

func (ff *flowFmt) Run(configFile string) error {
	switch configFile {
	case "-":
		if ff.write {
			return fmt.Errorf("cannot use -w with standard input")
		}
		return format("<stdin>", nil, os.Stdin, false)

	default:
		fi, err := os.Stat(configFile)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return fmt.Errorf("cannot format a directory")
		}

		f, err := os.Open(configFile)
		if err != nil {
			return err
		}
		defer f.Close()
		return format(configFile, fi, f, ff.write)
	}
}

func format(filename string, fi os.FileInfo, r io.Reader, write bool) error {
	bb, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(filename, bb)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, f); err != nil {
		return err
	}

	// Add a newline at the end of the file.
	_, _ = buf.Write([]byte{'\n'})

	if !write {
		_, err := io.Copy(os.Stdout, &buf)
		return err
	}

	wf, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, fi.Mode().Perm())
	if err != nil {
		return err
	}
	defer wf.Close()

	_, err = io.Copy(wf, &buf)
	return err
}
