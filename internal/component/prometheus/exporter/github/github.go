package github

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/github_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.github",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "github"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from Alloy.
var DefaultArguments = Arguments{
	APIURL: github_exporter.DefaultConfig.APIURL,
}

type Arguments struct {
	APIURL        string            `alloy:"api_url,attr,optional"`
	Repositories  []string          `alloy:"repositories,attr,optional"`
	Organizations []string          `alloy:"organizations,attr,optional"`
	Users         []string          `alloy:"users,attr,optional"`
	APIToken      alloytypes.Secret `alloy:"api_token,attr,optional"`
	APITokenFile  string            `alloy:"api_token_file,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *github_exporter.Config {
	return &github_exporter.Config{
		APIURL:        a.APIURL,
		Repositories:  a.Repositories,
		Organizations: a.Organizations,
		Users:         a.Users,
		APIToken:      config_util.Secret(a.APIToken),
		APITokenFile:  a.APITokenFile,
	}
}
