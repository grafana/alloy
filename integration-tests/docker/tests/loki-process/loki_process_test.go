//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestProcessLogFile(t *testing.T) {
	if err := common.WaitForInitalLogs(common.SanitizeTestName(t)); err != nil {
		t.Fatal(err)
	}

	common.AssertLogsPresent(
		t,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "cri",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "docker",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "json",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"format": "logfmt",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"method": "GET",
				"status": "200",
			},
			EntryCount: 1,
		},
	)

	common.AssertLabelsNotIndexed(t, "filename", "stream")
}
