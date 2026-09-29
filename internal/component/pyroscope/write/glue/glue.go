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
			if err := args.CheckStability(o.MinStability); err != nil {
				return nil, err
			}
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
			return &glue.GenericComponentGlue[write.Arguments]{Impl: &stabilityGated{Component: gc, minStability: o.MinStability}}, nil
		},
	})
}

// stabilityGated rejects updates that use features not permitted by the
// configured stability level.
type stabilityGated struct {
	*write.Component
	minStability featuregate.Stability
}

func (s *stabilityGated) Update(args write.Arguments) error {
	if err := args.CheckStability(s.minStability); err != nil {
		return err
	}
	return s.Component.Update(args)
}
