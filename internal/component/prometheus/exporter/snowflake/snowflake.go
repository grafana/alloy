package snowflake

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.snowflake",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "snowflake"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the snowflake exporter
var DefaultArguments = Arguments{
	Role: "ACCOUNTADMIN",
}

// Arguments controls the snowflake exporter.
type Arguments struct {
	AccountName string            `river:"account_name,attr"`
	Username    string            `river:"username,attr"`
	Password    rivertypes.Secret `river:"password,attr"`
	Role        string            `river:"role,attr,optional"`
	Warehouse   string            `river:"warehouse,attr"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *snowflake_exporter.Config {
	return &snowflake_exporter.Config{
		AccountName: a.AccountName,
		Username:    a.Username,
		Password:    config_util.Secret(a.Password),
		Role:        a.Role,
		Warehouse:   a.Warehouse,
	}
}
