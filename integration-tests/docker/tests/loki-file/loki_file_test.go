//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestReadLogFile(t *testing.T) {
	common.AssertLogsPresent(
		t,
		13,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"filename": "/etc/alloy/logs.txt",
			},
			EntryCount: 13,
		},
	)
}
