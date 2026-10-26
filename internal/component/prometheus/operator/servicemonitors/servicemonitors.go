package servicemonitors

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator/common"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.operator.servicemonitors",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			arguments := args.(Arguments)
			commonComponent, err := common.New(opts, arguments.Arguments, common.ServiceMonitorOptions(settingsFromArguments(arguments)))
			if err != nil {
				return nil, err
			}
			return &Component{Component: commonComponent}, nil
		},
	})
}

type Component struct {
	*common.Component
}

func (c *Component) Update(args component.Arguments) error {
	arguments := args.(Arguments)
	settings := settingsFromArguments(arguments)
	return c.Component.UpdateOperatorArguments(arguments.Arguments, &settings)
}

func settingsFromArguments(args Arguments) common.ServiceMonitorSettings {
	return common.ServiceMonitorSettings{
		AllowArbitraryFileAccess: args.AllowArbitraryFileAccess,
	}
}
