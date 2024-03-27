package probes

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/component/prometheus/operator/common"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.operator.probes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      operator.Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return common.New(opts, args, common.KindProbe)
		},
	})
}
