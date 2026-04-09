//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestLokiDocker(t *testing.T) {
	common.AssertLogsPresent(t, 12,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "producer-1",
			},
			EntryCount: 3,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "producer-2",
			},
			EntryCount: 3,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "producer-3",
			},
			EntryCount: 3,
		},
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "producer-4",
			},
			EntryCount: 3,
		},
	)
}
