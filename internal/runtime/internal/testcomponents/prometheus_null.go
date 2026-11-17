package testcomponents

import (
	"context"

	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.prometheus_null",
		Stability: featuregate.StabilityPublicPreview,
		Args:      PrometheusNullConfig{},
		Exports:   PrometheusNullExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			opts.OnStateChange(PrometheusNullExports{Receiver: appenders.Noop{}})
			return &PrometheusNull{}, nil
		},
	})
}

// PrometheusNullConfig configures the testcomponents.prometheus_null
// component. ForwardTo lets one instance reference another's receiver, which
// is useful for testing how references to Prometheus receivers are handled.
type PrometheusNullConfig struct {
	ForwardTo []storage.AppendableV2 `alloy:"forward_to,attr,optional"`
}

// PrometheusNullExports describes exported fields for the
// testcomponents.prometheus_null component.
type PrometheusNullExports struct {
	Receiver storage.AppendableV2 `alloy:"receiver,attr"`
}

// PrometheusNull implements the testcomponents.prometheus_null
// component. It discards everything it receives.
type PrometheusNull struct{}

var _ component.Component = (*PrometheusNull)(nil)

// Run implements Component.
func (r *PrometheusNull) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (r *PrometheusNull) Update(args component.Arguments) error {
	return nil
}
