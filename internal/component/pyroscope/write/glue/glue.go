package glue

import (
	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope/util/glue"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/useragent"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.write",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      write.Arguments{},
		Exports:   write.Exports{},
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			tracer := o.Tracer.Tracer("pyroscope.write")
			args := c.(write.Arguments)
			userAgent := useragent.Get()
			uid := alloyseed.Get().UID

			gc, err := write.New(
				o.Logger,
				tracer,
				o.Registerer,
				func(exports write.Exports) {
					o.OnStateChange(exports)
				},
				userAgent,
				uid,
				o.DataPath,
				args,
			)
			if err != nil {
				return nil, err
			}
			return &glue.GenericComponentGlue[write.Arguments]{Impl: gc}, nil
		},
	})
}
