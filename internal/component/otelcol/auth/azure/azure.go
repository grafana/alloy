package azure

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcol "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.auth.azure",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   auth.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := azureauthextension.NewFactory()
			return auth.New(opts, fact, args.(Arguments))
		},
	})
}

var (
	_ auth.Arguments   = Arguments{}
	_ syntax.Validator = Arguments{}
)

type Arguments struct {
	UseDefault       bool              `alloy:"use_default,attr,optional"`
	Scopes           []string          `alloy:"scopes,attr,optional"`
	ManagedIdentity  *ManagedIdentity  `alloy:"managed_identity,block,optional"`
	WorkloadIdentity *WorkloadIdentity `alloy:"workload_identity,block,optional"`
	ServicePrincipal *ServicePrincipal `alloy:"service_principal,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type ManagedIdentity struct {
	ClientID string `alloy:"client_id,attr,optional"`
}

type WorkloadIdentity struct {
	ClientID           string `alloy:"client_id,attr"`
	TenantID           string `alloy:"tenant_id,attr"`
	FederatedTokenFile string `alloy:"federated_token_file,attr"`
}

type ServicePrincipal struct {
	TenantID              string `alloy:"tenant_id,attr"`
	ClientID              string `alloy:"client_id,attr"`
	ClientSecret          string `alloy:"client_secret,attr,optional"`
	ClientCertificatePath string `alloy:"client_certificate_path,attr,optional"`
}

func (a Arguments) Validate() error {
	client, err := a.ConvertClient()
	if err != nil {
		return err
	}

	c := client.(*azureauthextension.Config)

	if err := c.Validate(); err != nil {
		return err
	}

	if c.Managed.HasValue() {
		if err := c.Managed.Get().Validate(); err != nil {
			return err
		}
	}

	if c.Workload.HasValue() {
		if err := c.Workload.Get().Validate(); err != nil {
			return err
		}
	}

	if c.ServicePrincipal.HasValue() {
		if err := c.ServicePrincipal.Get().Validate(); err != nil {
			return err
		}
	}

	return nil
}

// AuthFeatures implements auth.Arguments.
func (a Arguments) AuthFeatures() auth.AuthFeature {
	return auth.ClientAuthSupported
}

// ConvertClient implements auth.Arguments.
func (a Arguments) ConvertClient() (otelcomponent.Config, error) {
	c := azureauthextension.Config{
		UseDefault: a.UseDefault,
		Scopes:     a.Scopes,
	}

	if a.ManagedIdentity != nil {
		c.Managed = configoptional.Some(azureauthextension.ManagedIdentity{
			ClientID: a.ManagedIdentity.ClientID,
		})
	}

	if a.WorkloadIdentity != nil {
		c.Workload = configoptional.Some(azureauthextension.WorkloadIdentity{
			ClientID:           a.WorkloadIdentity.ClientID,
			TenantID:           a.WorkloadIdentity.TenantID,
			FederatedTokenFile: a.WorkloadIdentity.FederatedTokenFile,
		})
	}

	if a.ServicePrincipal != nil {
		c.ServicePrincipal = configoptional.Some(azureauthextension.ServicePrincipal{
			TenantID:              a.ServicePrincipal.TenantID,
			ClientID:              a.ServicePrincipal.ClientID,
			ClientSecret:          a.ServicePrincipal.ClientSecret,
			ClientCertificatePath: a.ServicePrincipal.ClientCertificatePath,
		})
	}

	return &c, nil
}

// ConvertServer implements auth.Arguments.
func (a Arguments) ConvertServer() (otelcomponent.Config, error) {
	// FIXME(kalleep): Supports server authentication in newer versions.
	return nil, nil
}

// DebugMetricsConfig implements auth.Arguments.
func (a Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return a.DebugMetrics
}

// Exporters implements auth.Arguments.
func (a Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Extensions implements auth.Arguments.
func (a Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}
