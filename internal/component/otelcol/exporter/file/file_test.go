package file_test

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/file"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
)

func Test(t *testing.T) {
	tests := []struct {
		testName string
		args string
		expectedReturn fileexporter.Config
		errorMsg string
	}{
		{
			testName: "defaultConfig",
			args: ``,
			expectedReturn: fileexporter.Config{
				// TODO"
			},
		},
		// invalid config
		// invalid path
		// invalid compression type
		// etc...
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			// Run the actual exporter...
			// Make sure it doesn't error (or does when we expect it to)
		})
	}
}
