package model

type RuntimeGraph struct {
	components    []Component
	modules       []Module
	componentByID map[string]int
	moduleByID    map[string]int
}

func NewRuntimeGraph(components []Component, modules []Module) *RuntimeGraph {
	graph := &RuntimeGraph{
		components:    components,
		modules:       modules,
		componentByID: make(map[string]int, len(components)),
		moduleByID:    make(map[string]int, len(modules)),
	}

	for i := range graph.components {
		graph.componentByID[graph.components[i].ID] = i
		graph.components[i].RuntimeGraph = graph
		for j := range graph.components[i].Edges.Incoming {
			graph.components[i].Edges.Incoming[j].RuntimeGraph = graph
		}
		for j := range graph.components[i].Edges.Outgoing {
			graph.components[i].Edges.Outgoing[j].RuntimeGraph = graph
		}
	}
	for i := range graph.modules {
		graph.moduleByID[graph.modules[i].ID] = i
		graph.modules[i].RuntimeGraph = graph
	}

	return graph
}

func (g *RuntimeGraph) Components() []Component {
	return append([]Component(nil), g.components...)
}

func (g *RuntimeGraph) Component(id string) *Component {
	index, found := g.componentByID[id]
	if !found {
		return nil
	}
	return &g.components[index]
}

func (g *RuntimeGraph) ComponentsByID(ids []string) []Component {
	components := make([]Component, 0, len(ids))
	for _, id := range ids {
		if component := g.Component(id); component != nil {
			components = append(components, *component)
		}
	}
	return components
}

func (g *RuntimeGraph) Modules() []Module {
	return append([]Module(nil), g.modules...)
}

func (g *RuntimeGraph) Module(id string) *Module {
	index, found := g.moduleByID[id]
	if !found {
		return nil
	}
	return &g.modules[index]
}

func (g *RuntimeGraph) ModulesByID(ids []string) []Module {
	modules := make([]Module, 0, len(ids))
	for _, id := range ids {
		if module := g.Module(id); module != nil {
			modules = append(modules, *module)
		}
	}
	return modules
}
