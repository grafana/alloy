package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	supervisortelemetry "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
)

const (
	envFleetManagementURL = "GCLOUD_FM_URL"
	envInstanceID         = "GCLOUD_INSTANCE_ID"
	envAPIToken           = "GCLOUD_RW_API_KEY"
	envStorageDir         = "STORAGE_DIR"
	envBasicAuthBase64    = "GCLOUD_BASIC_AUTH_BASE64"
)

const opAmpEndpoint = "/v1/opamp"

const svCmdDoc = `[EXPERIMENTAL] Run an embedded OpAMP supervisor that manages alloy as a supervised agent

Configuration can be provided in two ways:

1. Config File Mode:
   Pass the --config flag to point to a supervisor configuration file.

2. Simple Mode (Default):
   If --config is omitted, simple mode uses the following environment variables:

   GCLOUD_FM_URL
     Fleet management base URL (used as the OpAMP address).
     Note: The path "/v1/opamp" is appended if it's not already present.

   GCLOUD_INSTANCE_ID
     Grafana Cloud instance ID.

   GCLOUD_RW_API_KEY
     Grafana Cloud API token.

   STORAGE_DIR
     Path for supervisor storage directory.
`

func newOtelSupervisorCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:           "otel-supervisor",
		Short:         "Run the embedded OpAMP supervisor for Alloy's OTel engine",
		Long:          svCmdDoc,
		Hidden:        false,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSupervisor(configPath)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "path to a configuration file. If omitted, uses environment-based simple mode")

	return cmd
}

func runSupervisor(cfgPath string) error {
	cfg, creds, err := buildSupervisorConfig(cfgPath)
	if err != nil {
		return err
	}

	if creds != nil {
		// Expose auth credentials to the supervised collector process.
		// The FM collector health config uses this env var in its Basic auth header.
		if err := os.Setenv(envBasicAuthBase64, creds.basicAuthBase64); err != nil {
			return fmt.Errorf("cannot set %s environment variable: %w", envBasicAuthBase64, err)
		}
	}

	logger, err := supervisortelemetry.NewLogger(cfg.Telemetry.Logs)
	if err != nil {
		return fmt.Errorf("failed to build supervisor logger: %w", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Passed context should remain alive until graceful shutdown completes.
	// Cancelling the context doesn't shutdown the supervisor and can interrupt cleanup.
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup, err := newSupervisor(runCtx, logger, cfg)
	if err != nil {
		return err
	}

	if err := sup.Start(runCtx); err != nil {
		return fmt.Errorf("failed to start supervisor: %w", err)
	}

	interrupt := make(chan os.Signal, 2)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	<-interrupt
	sup.Shutdown()
	return nil
}

func newSupervisor(ctx context.Context, logger *zap.Logger, cfg *config.Supervisor) (*supervisor.Supervisor, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("unable to get executable path: %w", err)
	}

	if cfg.Agent.Executable != "" && cfg.Agent.Executable != exe {
		logger.Sugar().Warnf("warning: ignoring agent.executable %q from supervisor config; forcing the running Alloy binary %q", cfg.Agent.Executable, exe)
	}
	cfg.Agent.Executable = exe

	sup, err := supervisor.NewSupervisor(ctx, logger.Named("supervisor"), *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create supervisor: %w", err)
	}

	return sup, nil
}

func buildSupervisorConfig(cfgPath string) (*config.Supervisor, *authCredentials, error) {
	if cfgPath != "" {
		cfg, err := supervisorConfigFromFile(cfgPath)
		return cfg, nil, err
	}

	// Build config from env vars and Grafana Cloud defaults if config is unset.
	cfg, creds, err := supervisorConfigFromEnv()
	if err != nil {
		if e, ok := errors.AsType[*missingEnvVarsError](err); ok {
			return nil, nil, fmt.Errorf(
				"missing configuration: specify --config flag, or set the following environment variable(s): %s",
				strings.Join(e.missing, ", "),
			)
		}

		return nil, nil, err
	}

	return cfg, creds, nil
}

type missingEnvVarsError struct {
	missing []string
}

func (err *missingEnvVarsError) Error() string {
	return fmt.Sprintf("missing environment variables: %s", strings.Join(err.missing, ", "))
}

type authCredentials struct {
	basicAuthBase64 string
	apiToken        string
}

func supervisorConfigFromEnv() (*config.Supervisor, *authCredentials, error) {
	var missing []string
	fmURL := strings.TrimSpace(os.Getenv(envFleetManagementURL))
	if fmURL == "" {
		missing = append(missing, envFleetManagementURL)
	}

	instanceID := strings.TrimSpace(os.Getenv(envInstanceID))
	if instanceID == "" {
		missing = append(missing, envInstanceID)
	}

	token := strings.TrimSpace(os.Getenv(envAPIToken))
	if token == "" {
		missing = append(missing, envAPIToken)
	}

	storageDir := strings.TrimSpace(os.Getenv(envStorageDir))
	if storageDir == "" {
		missing = append(missing, envStorageDir)
	}

	if len(missing) > 0 {
		return nil, nil, &missingEnvVarsError{missing: missing}
	}

	// Append OpAMP endpoint if not defined
	fmURL = strings.TrimRight(fmURL, "/")
	if !strings.HasSuffix(fmURL, opAmpEndpoint) {
		fmURL += opAmpEndpoint
	}

	// Start from supervisor defaults and override only what simple mode needs.
	// Defaults already cover agent timeouts, orphan detection interval and "info" log level.
	cfg := config.DefaultSupervisor()
	cfg.Server.Endpoint = fmURL
	authStr := base64.StdEncoding.EncodeToString([]byte(instanceID + ":" + token))
	cfg.Server.Headers = http.Header{
		"Authorization": []string{"Basic " + authStr},
	}
	cfg.Capabilities.AcceptsRemoteConfig = true
	cfg.Capabilities.ReportsRemoteConfig = true
	cfg.Agent.Arguments = []string{"otel"}
	cfg.Agent.PassthroughLogs = true
	cfg.Storage.Directory = storageDir

	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid supervisor config: %w", err)
	}

	creds := &authCredentials{
		basicAuthBase64: authStr,
		apiToken:        token,
	}
	return &cfg, creds, nil
}

func supervisorConfigFromFile(cfgPath string) (*config.Supervisor, error) {
	if cfgPath == "" {
		return nil, fmt.Errorf("path to supervisor configuration file cannot be empty")
	}

	resolverSettings := confmap.ResolverSettings{
		DefaultScheme: "file",
		URIs:          []string{cfgPath},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
			envprovider.NewFactory(),
		},
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	if err != nil {
		return nil, err
	}

	conf, err := resolver.Resolve(context.Background())
	if err != nil {
		return nil, err
	}

	cfg := config.DefaultSupervisor()
	if err := conf.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid supervisor config %q: %w", cfgPath, err)
	}

	return &cfg, nil
}
