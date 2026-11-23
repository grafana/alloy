package pipelinetest

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

type TestConfig struct {
	DataPath string
	// WorkDir holds the files of the setup of a test. It replaces the
	// {{TEMP_DIR}} placeholder.
	WorkDir string
}

func RunTest(ctx context.Context, schema TestSchema, cfg TestConfig) error {
	if err := validateMocks(schema.Mocks); err != nil {
		return err
	}
	if err := validateHTTPInputs(schema.Inputs.HTTP); err != nil {
		return err
	}
	if err := validateSocketInputs(schema.Inputs.TCP, schema.Inputs.UDP); err != nil {
		return err
	}

	var timeout time.Duration
	if schema.Timeout != "" {
		var err error
		if timeout, err = time.ParseDuration(schema.Timeout); err != nil {
			return fmt.Errorf("parse timeout %q: %w", schema.Timeout, err)
		}
	}

	setup, err := runSetup(schema, cfg.WorkDir)
	defer setup.cleanup()
	if err != nil {
		return err
	}
	schema.Config = ConfigSchema(setup.config)

	alloy, err := harness.NewAlloy(harness.Config{
		SinkID:         "pipelinetest.sink.out",
		DataPath:       cfg.DataPath,
		Source:         withPrelude(schema),
		LokiPipelineV2: schema.Flags.LokiPipelineV2,
		AssertTimeout:  timeout,
	})
	if err != nil {
		return err
	}
	defer alloy.Stop()

	if err := produceInputs(ctx, alloy, schema.Inputs); err != nil {
		return err
	}

	assertions, err := buildAssertions(schema.Assert)
	if err != nil {
		return err
	}

	return alloy.Assert(assertions...)
}
