//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

func TestLokiCloudflare(t *testing.T) {
	common.AssertLogsPresent(t, 3,
		common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": "cloudflare",
			},
			EntryCount: 3,
		},
	)
}
