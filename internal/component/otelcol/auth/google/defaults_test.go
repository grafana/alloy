package google_test

import (
	"testing"

	gcpauth "github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
	"github.com/grafana/alloy/internal/component/otelcol/auth/google"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args google.Arguments
	args.SetToDefault()

	client, err := args.ConvertClient()
	require.NoError(t, err)
	// Literal, not factory-derived, so a contrib bump that changes a default fails here.
	require.Equal(t, &googleclientauthextension.Config{
		Config: gcpauth.Config{
			TokenType:   "access_token",
			TokenHeader: "authorization",
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/logging.write",
				"https://www.googleapis.com/auth/monitoring.write",
				"https://www.googleapis.com/auth/trace.append",
			},
		},
	}, client)
}
