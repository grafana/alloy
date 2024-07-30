// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "jaegerremotesampling"
)

// NewFactory creates a factory for the jaeger remote sampling extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		otelcomponent.MustNewType(typeStr),
		createDefaultConfig,
		createExtension,
		otelcomponent.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerConfig: &confighttp.ServerConfig{
			Endpoint: "localhost:5778",
		},
		GRPCServerConfig: &configgrpc.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  "localhost:14250",
				Transport: "tcp",
			},
		},
		Source: Source{},
	}
}

var once sync.Once

func logDeprecation(logger *zap.Logger) {
	once.Do(func() {
		logger.Warn("jaegerremotesampling extension will deprecate Thrift-gen and replace it with Proto-gen to be compatible with jaeger 1.42.0 and higher. See https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/18485 for more details.")
	})
}

// nolint
// var protoGate = featuregate.GlobalRegistry().MustRegister(
// 	"extension.jaegerremotesampling.replaceThriftWithProto",
// 	featuregate.StageStable,
// 	featuregate.WithRegisterDescription(
// 		"When enabled, the jaegerremotesampling will use Proto-gen over Thrift-gen.",
// 	),
// 	featuregate.WithRegisterToVersion("0.92.0"),
// )

func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	logDeprecation(set.Logger)
	return newExtension(cfg.(*Config), set.TelemetrySettings), nil
}
