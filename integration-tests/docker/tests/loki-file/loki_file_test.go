//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestReadLogFile(t *testing.T) {
	common.AssertLogsPresent(
		t,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"detected_level": "info",
			},
			EntryCount: 9,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"detected_level": "debug",
			},
			EntryCount: 1,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"detected_level": "error",
			},
			EntryCount: 2,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"detected_level": "warn",
			},
			EntryCount: 1,
		},
	)
}
