// Copyright Grafana Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	envprovider "go.opentelemetry.io/collector/confmap/provider/envprovider"
	fileprovider "go.opentelemetry.io/collector/confmap/provider/fileprovider"
	httpprovider "go.opentelemetry.io/collector/confmap/provider/httpprovider"
	httpsprovider "go.opentelemetry.io/collector/confmap/provider/httpsprovider"
	yamlprovider "go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/otelcol"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/configprovider/opampprovider"
)

func init() {
	opampprovider.ValidateRemoteConfig = func(ctx context.Context, remoteDir string) error {
		return validateRemoteConfig(ctx, remoteDir)
	}
}

func validateRemoteConfig(ctx context.Context, remoteDir string) error {
	var cps otelcol.ConfigProviderSettings
	if uris := opampprovider.StashedResolverURIsForValidation(); len(uris) > 0 {
		cps = otelcol.ConfigProviderSettings{
			ResolverSettings: resolverSettingsForValidation(uris),
		}
	} else {
		merged, merr := opampprovider.MergeRemoteYAMLFilesInDir(remoteDir)
		if merr != nil {
			return fmt.Errorf("merge remote config YAML: %w", merr)
		}
		if len(merged.ToStringMap()) == 0 {
			return nil
		}
		raw, yerr := yaml.Marshal(merged.ToStringMap())
		if yerr != nil {
			return fmt.Errorf("marshal remote merged config: %w", yerr)
		}
		cps = otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs: []string{"yaml:" + string(raw)},
				ProviderFactories: []confmap.ProviderFactory{
					yamlprovider.NewFactory(),
				},
			},
		}
	}

	col, err := otelcol.NewCollector(otelcol.CollectorSettings{
		BuildInfo: component.BuildInfo{
			Command:     "alloy",
			Description: "Alloy OTel Collector distribution.",
			Version:     CollectorVersion(),
		},
		Factories:              components,
		ConfigProviderSettings: cps,
	})
	if err != nil {
		return fmt.Errorf("collector: %w", err)
	}

	if err := col.DryRun(opampprovider.ContextWithoutRemoteWatch(ctx)); err != nil {
		return fmt.Errorf("dry run: %w", err)
	}
	return nil
}

func resolverSettingsForValidation(uris []string) confmap.ResolverSettings {
	return confmap.ResolverSettings{
		URIs: uris,
		ProviderFactories: []confmap.ProviderFactory{
			envprovider.NewFactory(),
			fileprovider.NewFactory(),
			httpprovider.NewFactory(),
			httpsprovider.NewFactory(),
			yamlprovider.NewFactory(),
			opampprovider.NewFactory(),
		},
	}
}
