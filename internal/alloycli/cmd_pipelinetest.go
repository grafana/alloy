package alloycli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/pipelinetest"
)

func testCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "test file",
		Short:         "Run a pipeline test schema",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runTestFile(args[0])
		},
	}

	return cmd
}

func runTestFile(path string) error {
	bb, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var schema pipelinetest.TestSchema
	if err := yaml.Unmarshal(bb, &schema); err != nil {
		return fmt.Errorf("unmarshal schema: %w", err)
	}

	if err := pipelinetest.RunTest(schema); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Pipeline test failed: ")
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return nil
}
