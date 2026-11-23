package pipelinetest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

type TestConfig struct {
	DataPath string
	// WorkDir holds the files of the setup of a test. It replaces the
	// {{TEMP_DIR}} placeholder.
	WorkDir string
	// Deadline is when go test stops the package. The assertion timeout
	// never goes past it, so a slow test fails with a message instead of
	// ending the package with a panic. The zero value means no deadline.
	Deadline time.Time
}

// deadlineMargin is the time to keep free before the deadline for cleanup.
const deadlineMargin = 30 * time.Second

// capTimeout makes sure a test ends before the deadline.
func capTimeout(timeout time.Duration, deadline time.Time) (time.Duration, error) {
	if deadline.IsZero() || timeout == 0 {
		return timeout, nil
	}
	remaining := time.Until(deadline) - deadlineMargin
	if remaining <= 0 {
		return 0, errors.New("not enough time left before the go test deadline")
	}
	return min(timeout, remaining), nil
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

	var stableFor time.Duration
	if schema.StableFor != "" {
		var err error
		if stableFor, err = time.ParseDuration(schema.StableFor); err != nil {
			return fmt.Errorf("parse stable_for %q: %w", schema.StableFor, err)
		}
	}

	timeout, err := capTimeout(timeout, cfg.Deadline)
	if err != nil {
		return err
	}

	config, err := runSetup(schema, cfg.WorkDir)
	if err != nil {
		return err
	}
	schema.Config = ConfigSchema(config)

	alloy, err := harness.NewAlloy(harness.Config{
		SinkID:         "pipelinetest.sink.out",
		DataPath:       cfg.DataPath,
		Source:         withPrelude(schema),
		LokiPipelineV2: schema.Flags.LokiPipelineV2,
		AssertTimeout:  timeout,
		StableFor:      stableFor,
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
