// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerremotesampling

import (
	"context"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	otelextension "go.opentelemetry.io/collector/extension"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "jaegerremotesampling"
)

// NewFactory creates a factory for the OIDC Authenticator extension.
func NewFactory() otelextension.Factory {
	return otelextension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		otelcomponent.StabilityLevelBeta,
	)
}

func createDefaultConfig() otelcomponent.Config {
	return &Config{
		HTTPServerSettings: &confighttp.HTTPServerSettings{
			Endpoint: ":5778",
		},
		GRPCServerSettings: &configgrpc.GRPCServerSettings{
			NetAddr: confignet.NetAddr{
				Endpoint:  ":14250",
				Transport: "tcp",
			},
		},
		Source: Source{},
	}
}

func createExtension(_ context.Context, set otelextension.CreateSettings, cfg otelcomponent.Config) (otelcomponent.Component, error) {
	return newExtension(cfg.(*Config), set.TelemetrySettings), nil
}
