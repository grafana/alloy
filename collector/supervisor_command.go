package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	supervisorTelemetry "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
	"github.com/spf13/cobra"
)

func registerOpAMPSupervisorCommand(flowCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "otel-supervisor",
		Short: "[EXPERIMENTAL] Run embedded OpAMP supervisor (supervises alloy otel subprocess)",
		Long:  "Loads an OpenTelemetry opampsupervisor-compatible YAML config and supervises the collector agent process (typically this binary with the otel subcommand).",
		RunE:  runOpAMPSupervisor,
	}
	cmd.Flags().String("config", "", "Path to a supervisor configuration file")
	_ = cmd.MarkFlagRequired("config")

	flowCmd.AddCommand(cmd)
}

func runOpAMPSupervisor(cmd *cobra.Command, _ []string) error {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load supervisor config: %w", err)
	}

	logger, err := supervisorTelemetry.NewLogger(cfg.Telemetry.Logs)
	if err != nil {
		return fmt.Errorf("failed to create supervisor logger: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup, err := supervisor.NewSupervisor(ctx, logger.Named("supervisor"), cfg)
	if err != nil {
		return fmt.Errorf("failed to create supervisor: %w", err)
	}

	if err := sup.Start(ctx); err != nil {
		return fmt.Errorf("failed to start supervisor: %w", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	sup.Shutdown()

	return nil
}
