package reviewer

import (
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
)

type Options struct {
	// Sources are all source files to review.
	Sources map[string][]byte
	// ComponentRegistry is used to get metadata of used components.
	ComponentRegistry component.Registry
}

func Review(opts Options) (*Result, error) {
	sources, err := runtime.ParseSources(opts.Sources)
	if err != nil {
		return nil, err
	}

	res := &Result{
		StabilityRequired: featuregate.StabilityGenerallyAvailable,
		Components:        make(map[string]Component),
	}

	for _, c := range sources.Components() {
		name := strings.Join(c.Name, ".")
		reg, err := opts.ComponentRegistry.Get(name)
		if err != nil {
			// TODO warn here?
			continue
		}

		if reg.Stability < res.StabilityRequired {
			res.StabilityRequired = reg.Stability
		}

		if _, ok := res.Components[name]; ok {
			continue
		}

		if reg.Metadata.IsEmpty() {
			continue
		}

		res.Components[name] = Component{
			Metadata: reg.Metadata,
		}
	}

	return res, nil
}

type Result struct {
	StabilityRequired featuregate.Stability
	Components        map[string]Component
}

type Component struct {
	Metadata component.Metadata
}
