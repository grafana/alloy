package alloycli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/pipelinetest"
)

// errTestsFailed is returned when tests ran but did not all pass. The report
// already explains why, so the command prints nothing more.
var errTestsFailed = errors.New("pipeline tests failed")

func pipelineTestCommand() *cobra.Command {
	t := &alloyPipelineTest{
		output:        "text",
		color:         "auto",
		assertTimeout: 30 * time.Second,
		match:         slices.Clone(pipelinetest.DefaultMatch),
	}

	cmd := &cobra.Command{
		Use:   "test [flags] [path ...]",
		Short: "[EXPERIMENTAL] Run pipeline tests against configuration files",
		Long: `The test subcommand runs declarative pipeline tests. EXPERIMENTAL: the
command, its flags, the test file format and the JSON output may change in
any release.

A test file is YAML with kind: pipelinetest and version: 1. It names an
Alloy configuration, the data to send into it, and assertions about what
comes out. To test a real configuration without changes, set the file path
and list the components to swap. Entry components become test inputs, and
exit components such as loki.write become test sinks.

Each path is a test file or a directory. A test file always runs. A
directory is searched recursively for files that match --match. A matched
file without kind: pipelinetest is skipped with a warning. With no path,
the current directory is searched.

While a test runs, paths in it and in its config are relative to the test
file's directory.

The command exits with status 0 when every test passes, and 1 otherwise.
Use --output=json for a machine-readable report, for CI or for an agent.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			t.matchSet = cmd.Flags().Changed("match")
			err := t.Run(cmd.Context(), args, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil && !errors.Is(err, errTestsFailed) {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err)
			}
			return err
		},
	}

	cmd.Flags().StringVarP(&t.output, "output", "o", t.output, "Output format: text or json.")
	cmd.Flags().StringVar(&t.color, "color", t.color, "Color text output: auto, always or never. auto colors a terminal unless NO_COLOR is set.")
	cmd.Flags().DurationVar(&t.assertTimeout, "timeout", t.assertTimeout, "How long each test retries its assertions before failing.")
	cmd.Flags().BoolVarP(&t.verbose, "verbose", "v", t.verbose, "Write Alloy's logs to stderr.")
	cmd.Flags().StringSliceVar(&t.match, "match", t.match, "Patterns that select test files in a directory. A pattern without a slash matches the file name. A pattern with a slash matches the path relative to the directory, and supports **. Can't be used with a file path.")

	return cmd
}

type alloyPipelineTest struct {
	output        string
	color         string
	assertTimeout time.Duration
	verbose       bool
	match         []string
	// matchSet is true when --match was given.
	matchSet bool
}

func (t *alloyPipelineTest) Run(ctx context.Context, paths []string, stdout, stderr io.Writer) error {
	if t.output != "text" && t.output != "json" {
		return fmt.Errorf("--output must be text or json, got %q", t.output)
	}
	useColor, err := t.useColor(stdout)
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}
	if t.matchSet {
		for _, p := range paths {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return fmt.Errorf("--match only applies to directories, but %s is a file", p)
			}
		}
	}
	found, err := pipelinetest.FindTestFiles(paths, t.match)
	if err != nil {
		return err
	}
	if len(found.Files) == 0 {
		return fmt.Errorf("no test files found in %v matching %v, with kind: %s (%d skipped)",
			paths, t.match, pipelinetest.Kind, len(found.Skipped))
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	opts := pipelinetest.RunOptions{AssertTimeout: t.assertTimeout}
	if t.verbose {
		opts.LogWriter = stderr
	}

	text := pipelinetest.TextReporter{W: stdout, Color: useColor}
	if t.output == "text" {
		opts.OnResult = text.File
		for _, sf := range found.Skipped {
			text.Skipped(sf)
		}
	}

	report := pipelinetest.RunFiles(ctx, found, opts)

	if t.output == "json" {
		if err := pipelinetest.WriteJSON(stdout, report); err != nil {
			return err
		}
	} else {
		text.Summary(report)
	}

	if !report.Passed() {
		return errTestsFailed
	}
	return nil
}

func (t *alloyPipelineTest) useColor(w io.Writer) (bool, error) {
	switch t.color {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "auto":
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			return false, nil
		}
		f, ok := w.(*os.File)
		if !ok {
			return false, nil
		}
		info, err := f.Stat()
		return err == nil && info.Mode()&os.ModeCharDevice != 0, nil
	default:
		return false, fmt.Errorf("--color must be auto, always or never, got %q", t.color)
	}
}
