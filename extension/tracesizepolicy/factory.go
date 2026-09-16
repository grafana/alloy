// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracesizepolicy

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

// Config is empty: the size threshold is set per policy, under that
// policy's `tracesizepolicy:` block, not on the extension instance itself.
type Config struct{}

var componentType = component.MustNewType("tracesizepolicy")

func NewFactory() extension.Factory {
	return extension.NewFactory(
		componentType,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelDevelopment,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(_ context.Context, _ extension.Settings, _ component.Config) (extension.Extension, error) {
	return newExtension(), nil
}
