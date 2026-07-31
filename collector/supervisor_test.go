package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupervisorConfigFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		fmURL        string
		wantEndpoint string
	}{
		{
			name:         "append OpAMP endpoint",
			fmURL:        " https://fleet-management.example.com/ ",
			wantEndpoint: "https://fleet-management.example.com/v1/opamp",
		},
		{
			name:         "keep OpAMP endpoint",
			fmURL:        "https://fleet-management.example.com/v1/opamp",
			wantEndpoint: "https://fleet-management.example.com/v1/opamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageDir := t.TempDir()
			t.Setenv(envFleetManagementURL, tt.fmURL)
			t.Setenv(envInstanceID, " 123 ")
			t.Setenv(envOTLPToken, " token ")
			t.Setenv(envStorageDir, storageDir)

			cfg, basicAuth, err := supervisorConfigFromEnv()
			require.NoError(t, err)
			require.Equal(t, tt.wantEndpoint, cfg.Server.Endpoint)
			require.Equal(t, "MTIzOnRva2Vu", basicAuth)
			require.Equal(t, "Basic "+basicAuth, cfg.Server.Headers.Get("Authorization"))
			require.True(t, cfg.Capabilities.AcceptsRemoteConfig)
			require.True(t, cfg.Capabilities.ReportsRemoteConfig)
			require.Equal(t, []string{"otel"}, cfg.Agent.Arguments)
			require.True(t, cfg.Agent.PassthroughLogs)
			require.Equal(t, storageDir, cfg.Storage.Directory)
		})
	}

	t.Run("missing env vars", func(t *testing.T) {
		t.Setenv(envFleetManagementURL, "")
		t.Setenv(envInstanceID, "")
		t.Setenv(envOTLPToken, "")
		t.Setenv(envStorageDir, "")

		cfg, basicAuth, err := supervisorConfigFromEnv()
		require.Nil(t, cfg)
		require.Empty(t, basicAuth)

		var missingErr *missingEnvVarsError
		require.ErrorAs(t, err, &missingErr)
		require.Equal(t, []string{
			envFleetManagementURL,
			envInstanceID,
			envOTLPToken,
			envStorageDir,
		}, missingErr.missing)
	})
}

func TestSupervisorConfigFromFile(t *testing.T) {
	t.Run("load config", func(t *testing.T) {
		t.Setenv("SUPERVISOR_TEST_ENDPOINT", "https://fleet-management.example.com/v1/opamp")

		cfgPath := filepath.Join("testdata", "supervisor.yaml")
		cfg, err := supervisorConfigFromFile(cfgPath)
		require.NoError(t, err)
		require.Equal(t, "https://fleet-management.example.com/v1/opamp", cfg.Server.Endpoint)
		require.Equal(t, "test-value", cfg.Server.Headers.Get("X-Test"))
		require.Equal(t, "/path/to/alloy", cfg.Agent.Executable)
		require.True(t, cfg.Agent.PassthroughLogs)
		require.Equal(t, []string{"otel"}, cfg.Agent.Arguments)
		require.Equal(t, "/tmp/supervisor", cfg.Storage.Directory)
		require.True(t, cfg.Capabilities.AcceptsRemoteConfig)
	})

	t.Run("empty path", func(t *testing.T) {
		cfg, err := supervisorConfigFromFile("")
		require.Nil(t, cfg)
		require.EqualError(t, err, "path to supervisor configuration file cannot be empty")
	})
}
