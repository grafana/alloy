package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/grafana/alloy/otel_engine/opampmanager"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/collector/otelcol"
)

const opampManagedConfigFlag = "opamp-managed-config"

func installOpAMPManagerHooks(otelCmd *cobra.Command, params otelcol.CollectorSettings) {
	otelCmd.PersistentFlags().String(
		opampManagedConfigFlag,
		"",
		"path to YAML for experimental OpAMP-managed effective config; when set, enables managed mode",
	)

	prev := otelCmd.PersistentPreRunE
	otelCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if prev != nil {
			if err := prev(cmd, args); err != nil {
				return err
			}
		}
		cfg, err := loadOpAMPManagerConfig(cmd)
		if err != nil {
			return err
		}
		if !cfg.Enabled {
			return nil
		}
		if err := opampmanager.BootReconcile(cfg.EffectivePath, cfg.StatePath, log.Default()); err != nil {
			return err
		}
		cfgURIs, setURIs, err := otelConfigResolverParts(cmd)
		if err != nil {
			return fmt.Errorf("opampmanager: CLI config locations: %w", err)
		}
		opampmanager.Start(cmd.Context(), cfg, params, cfgURIs, setURIs, log.Default())
		return nil
	}
}

func loadOpAMPManagerConfig(cmd *cobra.Command) (opampmanager.Config, error) {
	p, err := cmd.Flags().GetString(opampManagedConfigFlag)
	if err != nil {
		return opampmanager.Config{}, err
	}
	if strings.TrimSpace(p) != "" {
		return opampmanager.LoadManagedConfigFromPath(p)
	}
	return opampmanager.Config{}, nil
}
