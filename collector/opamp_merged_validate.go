// Copyright Grafana Labs and OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	opamp "github.com/grafana/alloy/configprovider/opamp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	envprovider "go.opentelemetry.io/collector/confmap/provider/envprovider"
	fileprovider "go.opentelemetry.io/collector/confmap/provider/fileprovider"
	httpprovider "go.opentelemetry.io/collector/confmap/provider/httpprovider"
	httpsprovider "go.opentelemetry.io/collector/confmap/provider/httpsprovider"
	yamlprovider "go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/otelcol"
)

const dryRunMergedYAMLTimeout = 60 * time.Second

func init() {
	opamp.ValidateMergedYAML = dryRunMergedYAML
}

func dryRunValidationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := parent.Deadline(); hasDeadline {
		return parent, func() {}
	}
	return context.WithTimeout(parent, dryRunMergedYAMLTimeout)
}

func yamlOnlyProviderFactories() []confmap.ProviderFactory {
	return []confmap.ProviderFactory{
		envprovider.NewFactory(),
		fileprovider.NewFactory(),
		httpprovider.NewFactory(),
		httpsprovider.NewFactory(),
		yamlprovider.NewFactory(),
	}
}

func dryRunMergedYAML(ctx context.Context, yaml []byte) error {
	vctx, cancel := dryRunValidationContext(ctx)
	defer cancel()

	build := component.BuildInfo{
		Command:     "alloy",
		Description: "Alloy OTel Collector distribution.",
		Version:     CollectorVersion(),
	}

	rs := confmap.ResolverSettings{
		URIs:              []string{"yaml:" + string(yaml)},
		ProviderFactories: yamlOnlyProviderFactories(),
	}

	set := otelcol.CollectorSettings{
		BuildInfo: build,
		Factories: components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: rs,
		},
	}

	col, err := otelcol.NewCollector(set)
	if err != nil {
		return fmt.Errorf("opamp validate: new collector: %w", err)
	}
	if err := col.DryRun(vctx); err != nil {
		return fmt.Errorf("opamp validate: %w", err)
	}
	return nil
}
