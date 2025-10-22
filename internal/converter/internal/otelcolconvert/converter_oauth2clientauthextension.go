package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/auth/oauth2"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, oauth2ClientAuthExtensionConverter{})
}

type oauth2ClientAuthExtensionConverter struct{}

func (oauth2ClientAuthExtensionConverter) Factory() component.Factory {
	return oauth2clientauthextension.NewFactory()
}

func (oauth2ClientAuthExtensionConverter) InputComponentName() string { return "otelcol.auth.oauth2" }

func (oauth2ClientAuthExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOAuth2ClientAuthExtension(cfg.(*oauth2clientauthextension.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "auth", "oauth2"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOAuth2ClientAuthExtension(cfg *oauth2clientauthextension.Config) *oauth2.Arguments {
	return &oauth2.Arguments{
		ClientID:       cfg.ClientID,
		ClientSecret:   alloytypes.Secret(cfg.ClientSecret),
		TokenURL:       cfg.TokenURL,
		EndpointParams: cfg.EndpointParams,
		Scopes:         cfg.Scopes,
		TLS:            toTLSClientArguments(cfg.TLS),
		Timeout:        cfg.Timeout,
		DebugMetrics:   common.DefaultValue[oauth2.Arguments]().DebugMetrics,
	}
}
