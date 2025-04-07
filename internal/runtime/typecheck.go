package runtime

import (
	"fmt"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/tracing"
)

func TypeCheck(s *Source, o Options) error {
	// FIXME: Do I need to use the loader controller???
	var (
		logger = o.Logger
		tracer = o.Tracer
	)

	if tracer == nil {
		var err error
		tracer, err = tracing.New(tracing.DefaultOptions)
		if err != nil {
			// This shouldn't happen unless there's a bug
			panic(err)
		}
	}

	if logger == nil {
		logger = logging.NewNop()
	}

	serviceMap := controller.NewServiceMap(o.Services)

	loader := controller.NewLoader(controller.LoaderOptions{
		ComponentGlobals: controller.ComponentGlobals{
			ControllerID:  "",
			Logger:        logger,
			TraceProvider: tracer,
			Registerer:    o.Reg,
			MinStability:  featuregate.StabilityExperimental,
			OnBlockNodeUpdate: func(cn controller.BlockNode) {
				fmt.Println("OnBlockUpdate")
			},
			OnExportsChange: o.OnExportsChange,
			NewModuleController: func(opts controller.ModuleControllerOpts) controller.ModuleController {
				return newModuleController(&moduleControllerOptions{
					ModuleRegistry:       newModuleRegistry(),
					Logger:               logger,
					Tracer:               tracer,
					Reg:                  o.Reg,
					DataPath:             o.DataPath,
					MinStability:         o.MinStability,
					EnableCommunityComps: o.EnableCommunityComps,
					ID:                   opts.Id,
					ServiceMap:           nil,
					WorkerPool:           nil,
				})
			},
			GetServiceData: func(name string) (any, error) {
				svc, found := serviceMap.Get(name)
				if !found {
					return nil, fmt.Errorf("service %q does not exist", name)
				}
				return svc.Data(), nil

			},
			EnableCommunityComps: o.EnableCommunityComps,
		},

		// FIXME: What should we set these too
		Host:              nil,
		ComponentRegistry: nil,
		WorkerPool:        nil,
		Services:          o.Services,
		EvalMode:          controller.EvalModeTypeCheck,
	})

	err := loader.Apply(controller.ApplyOptions{
		ComponentBlocks: s.components,
		ConfigBlocks:    s.configBlocks,
		DeclareBlocks:   s.declareBlocks,
		// FIXME(kalleep): what should i set these to
		Args:                    nil,
		CustomComponentRegistry: nil,
		ArgScope:                nil,
	},
	)
	if err != nil {
		return err
	}

	return nil
}
