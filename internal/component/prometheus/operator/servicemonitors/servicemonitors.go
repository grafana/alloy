package servicemonitors

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/operator"
	"github.com/grafana/agent/internal/component/prometheus/operator/common"
	"github.com/grafana/agent/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.operator.servicemonitors",
		Stability: featuregate.StabilityStable,
		Args:      operator.Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return common.New(opts, args, common.KindServiceMonitor)
		},
	})
}
