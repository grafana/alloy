//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestReadCompressedFiles(t *testing.T) {
	common.AssertLogsPresent(
		t,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"compression": "gz",
			},
			EntryCount: 300,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"compression": "z",
			},
			EntryCount: 300,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"compression": "bz2",
			},
			EntryCount: 300,
		},
	)
}
