package write

import (
	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/pyroscope/util/glue"
	"github.com/grafana/alloy/internal/component/pyroscope/write/writer"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/useragent"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.write",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      writer.Arguments{},
		Exports:   writer.Exports{},
		Build: func(o component.Options, c component.Arguments) (component.Component, error) {
			tracer := o.Tracer.Tracer("pyroscope.write")
			args := c.(writer.Arguments)
			userAgent := useragent.Get()
			uid := alloyseed.Get().UID

			gc, err := writer.New(
				o.Logger,
				tracer,
				o.Registerer,
				func(exports writer.Exports) {
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
			return &glue.GenericComponentGlue[writer.Arguments]{Impl: gc}, nil
		},
	})
}
