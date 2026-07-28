package main

import (

	// TODO: this imported to just download dependencies. Bare imports will be removed after newOtelSupervisorCommand is finished.
	"context"
	"errors"
	"fmt"
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

	"github.com/spf13/cobra"
)

const (
	// envFleetManagementURL is the base Fleet Management URL (no path); "/v1/opamp" is appended.
	envFleetManagementURL = "GCLOUD_FM_URL"

	// envBasicAuth is the pre-encoded Basic credential, base64(instance_id:token).
	envBasicAuth = "GCLOUD_BASIC_AUTH"

	// envStorageDir defines the supervisor storage directory.
	envStorageDir = "STORAGE_DIR"
)

const svCmdDoc = `[EXPERIMENTAL] Run an embedded OpAMP supervisor that manages alloy as a supervised agent

Configuration can be provided in two ways:

1. Config File Mode:
   Pass the --config flag to point to a supervisor configuration file.

2. Simple Mode (Default):
   If --config is omitted, simple mode uses the following environment variables:

   GCLOUD_FM_URL
     Fleet management base URL (used as the OpAMP address).
     Note: The path "/v1/opamp" is appended if it's not already present.

   GCLOUD_BASIC_AUTH
     Pre-encoded Basic auth credentials in base64 format (instance_id:token).

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
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to get executable path: %w", err)
	}

	var cfg *config.Supervisor
	if cfgPath == "" {
		// Build config from env vars and Grafana Cloud defaults if config is unset.
		cfg, err = supervisorConfigFromEnv()
	} else {
		cfg, err = supervisorConfigFromFile(cfgPath)
	}

	if err != nil {
		if e, ok := errors.AsType[*missingEnvVarsError](err); ok {
			return fmt.Errorf(
				"missing configuration: specify --config flag, or set the following environment variable(s): %s",
				strings.Join(e.missing, ", "),
			)
		}

		return err
	}

	logger, err := supervisortelemetry.NewLogger(cfg.Telemetry.Logs)
	if err != nil {
		return fmt.Errorf("failed to build supervisor logger: %w", err)
	}
	defer logger.Sync()

	if cfg.Agent.Executable != "" && cfg.Agent.Executable != exe {
		logger.Sugar().Warnf("warning: ignoring agent.executable %q from supervisor config; forcing the running Alloy binary %q\n", cfg.Agent.Executable, exe)
	}
	cfg.Agent.Executable = exe

	// Passed context should remain alive until graceful shutdown completes.
	// Cancelling the context doesn't shutdown the supervisor and can interrupt cleanup.
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup, err := supervisor.NewSupervisor(runCtx, logger.Named("supervisor"), *cfg)
	if err != nil {
		return fmt.Errorf("failed to create supervisor: %w", err)
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

type missingEnvVarsError struct {
	missing []string
}

func (err *missingEnvVarsError) Error() string {
	return fmt.Sprintf("missing environment variables: %s", strings.Join(err.missing, ", "))
}

const opAmpEndpoint = "/v1/opamp"

func supervisorConfigFromEnv() (*config.Supervisor, error) {
	var missing []string
	fmURL := strings.TrimSpace(os.Getenv(envFleetManagementURL))
	if fmURL == "" {
		missing = append(missing, envFleetManagementURL)
	}

	authStr := strings.Join(strings.Fields(os.Getenv(envBasicAuth)), "")
	if authStr == "" {
		missing = append(missing, envBasicAuth)
	}

	storageDir := os.Getenv(envStorageDir)
	if storageDir == "" {
		missing = append(missing, envStorageDir)
	}

	if len(missing) > 0 {
		return nil, &missingEnvVarsError{missing: missing}
	}

	// Append OpAMP endpoint if not defined
	// TODO: should this be URL parse + Path set?
	fmURL = strings.TrimRight(fmURL, "/")
	if !strings.HasSuffix(fmURL, opAmpEndpoint) {
		fmURL += opAmpEndpoint
	}

	// TODO: Modify config.Supervisor instead of unmarshal
	cfgMap := map[string]any{
		"server": map[string]any{
			"endpoint": fmURL,
			"headers": map[string]any{
				"Authorization": "Basic " + authStr,
			},
		},
		"capabilities": map[string]any{
			"accepts_remote_config": true,
			"reports_remote_config": true,
		},
		"agent": map[string]any{
			"args":                      []any{"otel"},
			"passthrough_logs":          true,
			"orphan_detection_interval": "5s",
		},
		"storage": map[string]any{
			"directory": storageDir,
		},
		"telemetry": map[string]any{
			"logs": map[string]any{
				"level": "info",
			},
		},
	}

	cfg := config.DefaultSupervisor()
	err := confmap.NewFromStringMap(cfgMap).Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build supervisor config: %w", err)
	}

	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("cannot validate generated supervisor config: %w", err)
	}

	return &cfg, nil
}

func supervisorConfigFromFile(cfgPath string) (*config.Supervisor, error) {
	if cfgPath == "" {
		return nil, fmt.Errorf("path to supervisor configuration file cannot be empty")
	}

	resolverSettings := confmap.ResolverSettings{
		DefaultScheme: "env",
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
