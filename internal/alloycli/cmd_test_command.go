package alloycli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/pipelinetest"
	"github.com/grafana/alloy/internal/statictest"
)

// errTestsFailed is returned when tests ran but did not all pass. The report
// already explains why, so the command prints nothing more.
var errTestsFailed = errors.New("tests failed")

const testLongCommon = `EXPERIMENTAL: the commands, their flags, the test file formats and the
JSON output may change in any release.

Each path is a test file or a directory. A test file always runs. A
directory is searched recursively for files that match --match. A matched
file without a test kind is skipped with a warning. With no path, the
current directory is searched.

The command exits with status 0 when every test passes, and 1 otherwise.
Use --output=json for a machine-readable report, for CI or for an agent.`

func testCommand() *cobra.Command {
	t := &alloyTest{
		output:        "text",
		color:         "auto",
		assertTimeout: 30 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "test [flags] [path ...]",
		Short: "[EXPERIMENTAL] Run pipeline and static tests against configuration files",
		Long: `The test subcommand runs pipeline tests and static tests. With no
subcommand, it runs both kinds.

A pipeline test (kind: pipelinetest, in *.pipelinetest.yaml) runs a
configuration, sends it data, and checks what comes out. A static test
(kind: statictest, in *.statictest.yaml) checks a configuration against
rules, without running it.

` + testLongCommon,
		Args:          cobra.ArbitraryArgs,
		RunE:          t.runE(pipelinetest.AllKinds),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "pipeline [flags] [path ...]",
		Short: "Run pipeline tests",
		Long: `The pipeline subcommand runs pipeline tests. A pipeline test is YAML with
kind: pipelinetest. It names an Alloy configuration, the data to send into
it, and assertions about what comes out. To test a real configuration
without changes, set the file path and list the components to swap.

While a test runs, paths in it and in its config are relative to the test
file's directory.

` + testLongCommon,
		RunE:          t.runE([]string{pipelinetest.Kind}),
		SilenceUsage:  true,
		SilenceErrors: true,
	}, &cobra.Command{
		Use:   "static [flags] [path ...]",
		Short: "Run static tests",
		Long: `The static subcommand runs static tests. A static test is YAML with kind:
statictest. It names an Alloy configuration, and lists rules that the
configuration must follow, such as clustering on every scrape component.
The configuration doesn't run, so a static test takes milliseconds.

A failed rule with severity: should is a warning, and doesn't fail the
test.

` + testLongCommon,
		RunE:          t.runE([]string{statictest.Kind}),
		SilenceUsage:  true,
		SilenceErrors: true,
	})

	flags := cmd.PersistentFlags()
	flags.StringVarP(&t.output, "output", "o", t.output, "Output format: text or json.")
	flags.StringVar(&t.color, "color", t.color, "Color text output: auto, always or never. auto colors a terminal unless NO_COLOR is set.")
	flags.DurationVar(&t.assertTimeout, "timeout", t.assertTimeout, "How long each pipeline test retries its assertions before failing.")
	flags.BoolVarP(&t.verbose, "verbose", "v", t.verbose, "Write Alloy's logs from pipeline tests to stderr.")
	flags.StringSliceVar(&t.match, "match", nil, "Patterns that select test files in a directory. A pattern without a slash matches the file name. A pattern with a slash matches the path relative to the directory, and supports **. Can't be used with a file path. (default *.<kind>.yaml and *.<kind>.yml for each kind the command runs)")

	return cmd
}

func (t *alloyTest) runE(kinds []string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		t.kinds = kinds
		t.matchSet = cmd.Flag("match").Changed
		err := t.Run(cmd.Context(), args, cmd.OutOrStdout(), cmd.ErrOrStderr())
		if err != nil && !errors.Is(err, errTestsFailed) {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err)
		}
		return err
	}
}

type alloyTest struct {
	output        string
	color         string
	assertTimeout time.Duration
	verbose       bool
	match         []string
	// matchSet is true when --match was given.
	matchSet bool
	// kinds are the test kinds to run.
	kinds []string
}

func (t *alloyTest) Run(ctx context.Context, paths []string, stdout, stderr io.Writer) error {
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
	kinds := t.kinds
	if len(kinds) == 0 {
		kinds = pipelinetest.AllKinds
	}
	match := t.match
	if !t.matchSet {
		match = pipelinetest.DefaultMatch(kinds...)
	}
	found, err := pipelinetest.FindTestFiles(paths, match, kinds)
	if err != nil {
		return err
	}
	if len(found.Files) == 0 {
		return fmt.Errorf("no test files found in %v matching %v, with kind: %s (%d skipped)",
			paths, match, strings.Join(kinds, " or "), len(found.Skipped))
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	opts := pipelinetest.RunOptions{AssertTimeout: t.assertTimeout, Kinds: kinds}
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

func (t *alloyTest) useColor(w io.Writer) (bool, error) {
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
