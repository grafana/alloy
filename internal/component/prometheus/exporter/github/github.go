package github

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/github_exporter"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.github",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "github"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from river.
var DefaultArguments = Arguments{
	APIURL: github_exporter.DefaultConfig.APIURL,
}

type Arguments struct {
	APIURL        string            `river:"api_url,attr,optional"`
	Repositories  []string          `river:"repositories,attr,optional"`
	Organizations []string          `river:"organizations,attr,optional"`
	Users         []string          `river:"users,attr,optional"`
	APIToken      rivertypes.Secret `river:"api_token,attr,optional"`
	APITokenFile  string            `river:"api_token_file,attr,optional"`
}

// SetToDefault implements river.Defaulter.
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
