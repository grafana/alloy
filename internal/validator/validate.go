package validator

import (
	"strings"

	"github.com/grafana/alloy/internal/component"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/service"

	"github.com/grafana/alloy/syntax/ast"
)

type Options struct {
	// ServiceDefinitions is used to validate service config.
	ServiceDefinitions []service.Definition
	// ComponentRegistry is used to validate component config.
	ComponentRegistry component.Registry
}

func Validate(sources map[string][]byte, opts Options) error {
	source, err := alloy_runtime.ParseSources(sources)
	if err != nil {
		return err
	}

	components, _ := splitComponents(source.Components(), opts.ServiceDefinitions)

	diags := validateComponents(components, opts.ComponentRegistry)
	if diags != nil {
		return diags
	}

	return nil
}

func splitComponents(blocks []*ast.BlockStmt, serviceDefinitions []service.Definition) ([]*ast.BlockStmt, []*ast.BlockStmt) {
	names := make(map[string]struct{}, len(serviceDefinitions))
	for _, def := range serviceDefinitions {
		names[def.Name] = struct{}{}
	}

	components := make([]*ast.BlockStmt, 0, len(blocks))
	services := make([]*ast.BlockStmt, 0, len(serviceDefinitions))

	for _, b := range blocks {
		if _, isService := names[BlockID(b)]; isService {
			services = append(services, b)
		} else {
			components = append(components, b)
		}
	}

	return components, services
}

func BlockID(b *ast.BlockStmt) string {
	id := make([]string, 0, len(b.Name)+1)
	id = append(id, b.Name...)
	if b.Label != "" {
		id = append(id, b.Label)
	}
	return strings.Join(id, ".")
}
