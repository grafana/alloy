package google_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/auth/google"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args google.Arguments
	args.SetToDefault()

	client, err := args.ConvertClient()
	require.NoError(t, err)
	cfg := client.(*googleclientauthextension.Config)

	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	// The embedded config's type moved to an upstream internal package in v0.161, so this
	// covers its fields rather than the whole struct. A new upstream field would slip past.
	require.Equal(t, "", cfg.Config.Project)
	require.Equal(t, "", cfg.Config.QuotaProject)
	require.Equal(t, "access_token", cfg.Config.TokenType)
	require.Equal(t, "", cfg.Config.Audience)
	require.Equal(t, "authorization", cfg.Config.TokenHeader)
	require.Equal(t, []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/logging.write",
		"https://www.googleapis.com/auth/monitoring.write",
		"https://www.googleapis.com/auth/trace.append",
	}, cfg.Config.Scopes)
}
