package opampmanager

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/otelcol"
)

func ValidateOTelYAML(ctx context.Context, baseSettings otelcol.CollectorSettings, validateURIs []string) error {
	if len(validateURIs) == 0 {
		return fmt.Errorf("opampmanager: no URIs for validate")
	}

	vs := baseSettings
	vs.ConfigProviderSettings.ResolverSettings.URIs = append([]string(nil), validateURIs...)
	if vs.ConfigProviderSettings.ResolverSettings.DefaultScheme == "" {
		vs.ConfigProviderSettings.ResolverSettings.DefaultScheme = "env"
	}

	col, err := otelcol.NewCollector(vs)
	if err != nil {
		return fmt.Errorf("opampmanager: new collector for validate: %w", err)
	}
	return col.DryRun(ctx)
}
