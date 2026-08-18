package main

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Valid values for every env var consumed by simple mode.
var validSupervisorEnv = map[string]string{
	envFleetManagementURL: " https://fleet-management.example.com/ ",
	envInstanceID:         " 123 ",
	envAPIToken:           " token ",
	envStorageDir:         "/tmp/supervisor",
}

// setSupervisorEnv populates simple mode env vars, blanking the ones in missing.
func setSupervisorEnv(t *testing.T, missing ...string) {
	t.Helper()

	blank := make(map[string]struct{}, len(missing))
	for _, name := range missing {
		blank[name] = struct{}{}
	}

	for _, name := range supervisorEnvVars {
		value := validSupervisorEnv[name]
		if _, ok := blank[name]; ok {
			value = ""
		}
		t.Setenv(name, value)
	}
}

// Declared separately from validSupervisorEnv to keep the order the missing var error reports.
var supervisorEnvVars = []string{
	envFleetManagementURL,
	envInstanceID,
	envAPIToken,
	envStorageDir,
}

func TestBuildSupervisorConfigFromEnv(t *testing.T) {
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
			setSupervisorEnv(t)
			t.Setenv(envFleetManagementURL, tt.fmURL)
			t.Setenv(envStorageDir, storageDir)

			cfg, creds, err := buildSupervisorConfig("")
			require.NoError(t, err)
			require.Equal(t, tt.wantEndpoint, cfg.Server.Endpoint)
			require.NotNil(t, creds)
			require.Equal(t, "MTIzOnRva2Vu", creds.basicAuthBase64)
			require.Equal(t, "token", creds.apiToken)
			require.Equal(t, "Basic "+creds.basicAuthBase64, cfg.Server.Headers.Get("Authorization"))
			require.True(t, cfg.Capabilities.AcceptsRemoteConfig)
			require.True(t, cfg.Capabilities.ReportsRemoteConfig)
			require.Equal(t, []string{"otel"}, cfg.Agent.Arguments)
			require.True(t, cfg.Agent.PassthroughLogs)
			require.Equal(t, storageDir, cfg.Storage.Directory)
		})
	}
}

func TestBuildSupervisorConfigMissingEnv(t *testing.T) {
	const errPrefix = "missing configuration: specify --config flag, or set the following environment variable(s): "

	// Each var alone must be reported, so a partially configured environment
	// still names exactly what's left to set.
	for _, name := range supervisorEnvVars {
		t.Run("missing "+name, func(t *testing.T) {
			setSupervisorEnv(t, name)

			cfg, creds, err := buildSupervisorConfig("")
			require.Nil(t, cfg)
			require.Nil(t, creds)
			require.EqualError(t, err, errPrefix+name)
		})
	}

	t.Run("missing all", func(t *testing.T) {
		setSupervisorEnv(t, supervisorEnvVars...)

		cfg, creds, err := buildSupervisorConfig("")
		require.Nil(t, cfg)
		require.Nil(t, creds)
		require.EqualError(t, err, errPrefix+"GCLOUD_FM_URL, GCLOUD_INSTANCE_ID, GCLOUD_RW_API_KEY, STORAGE_DIR")
	})
}

func TestBuildSupervisorConfigFromFile(t *testing.T) {
	t.Run("load config", func(t *testing.T) {
		t.Setenv("SUPERVISOR_TEST_ENDPOINT", "https://fleet-management.example.com/v1/opamp")

		cfgPath := filepath.Join("testdata", "supervisor.yaml")
		cfg, creds, err := buildSupervisorConfig(cfgPath)
		require.NoError(t, err)
		// Config file mode carries no credentials to pass to the agent.
		require.Nil(t, creds)
		require.Equal(t, "https://fleet-management.example.com/v1/opamp", cfg.Server.Endpoint)
		require.Equal(t, "test-value", cfg.Server.Headers.Get("X-Test"))
		require.Equal(t, "/path/to/alloy", cfg.Agent.Executable)
		require.True(t, cfg.Agent.PassthroughLogs)
		require.Equal(t, []string{"otel"}, cfg.Agent.Arguments)
		require.Equal(t, "/tmp/supervisor", cfg.Storage.Directory)
		require.True(t, cfg.Capabilities.AcceptsRemoteConfig)
	})

	t.Run("invalid config", func(t *testing.T) {
		cfgPath := filepath.Join("testdata", "supervisor_invalid.yaml")
		cfg, creds, err := buildSupervisorConfig(cfgPath)
		require.Nil(t, cfg)
		require.Nil(t, creds)
		require.ErrorContains(t, err, "invalid supervisor config "+strconv.Quote(cfgPath))
		require.ErrorContains(t, err, "not configured under extensions")
	})

	t.Run("missing file", func(t *testing.T) {
		cfgPath := filepath.Join("testdata", "does_not_exist.yaml")
		cfg, creds, err := buildSupervisorConfig(cfgPath)
		require.Nil(t, cfg)
		require.Nil(t, creds)
		require.Error(t, err)
	})
}

func TestSupervisorConfigFromFileEmptyPath(t *testing.T) {
	// Not reachable through buildSupervisorConfig: an empty path selects env mode.
	cfg, err := supervisorConfigFromFile("")
	require.Nil(t, cfg)
	require.EqualError(t, err, "path to supervisor configuration file cannot be empty")
}
