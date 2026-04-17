package pipelinetest

import (
	"os"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
)

func RunTest(schema TestSchema) error {
	dataPath, err := os.MkdirTemp("", "alloy-pipelinetest-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataPath)

	alloy, err := harness.NewAlloy(harness.Config{
		SinkID:   "pipelinetest.sink.out",
		DataPath: dataPath,
		Source:   withPrelude(schema),
	})
	if err != nil {
		return err
	}
	defer alloy.Stop()

	if err := produceInputs(alloy, schema.Inputs); err != nil {
		return err
	}

	assertions, err := buildAssertions(schema.Assert)
	if err != nil {
		return err
	}

	return alloy.Assert(assertions...)
}
