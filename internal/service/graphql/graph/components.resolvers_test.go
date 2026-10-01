package graph

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	graphqlruntime "github.com/grafana/alloy/internal/service/graphql/runtime"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func TestComponentEdgeDataRatesDefaultsToFullSnapshotWindow(t *testing.T) {
	snapshotter := &recordingRateSnapshotter{}
	resolver := &componentEdgeResolver{Resolver: &Resolver{CallbackManager: snapshotter}}

	_, err := resolver.DataRates(context.Background(), &model.ComponentEdge{}, nil)

	require.NoError(t, err)
	require.Equal(t, 60, snapshotter.windowSeconds)
}

func TestModuleHierarchyResolvers(t *testing.T) {
	const (
		ownerID       = "custom.pipeline.default"
		childModuleID = ownerID
		childID       = childModuleID + "/prometheus.scrape.default"
	)
	owner := &component.Info{
		ID:            component.ParseID(ownerID),
		ComponentName: "custom.pipeline",
		ModuleIDs:     []string{childModuleID},
	}
	child := &component.Info{
		ID:            component.ParseID(childID),
		ComponentName: "prometheus.scrape",
	}
	runtimeGraph := graphqlruntime.NewGraph([]*component.Info{owner, child})
	componentResolver := &componentResolver{}
	moduleResolver := &moduleResolver{}

	ownerModel := runtimeGraph.Component(ownerID)
	parentModule, err := componentResolver.ParentModule(context.Background(), ownerModel)
	require.NoError(t, err)
	require.Equal(t, "", parentModule.ID)

	childModules, err := componentResolver.ChildModules(context.Background(), ownerModel)
	require.NoError(t, err)
	require.Equal(t, []string{childModuleID}, moduleIDs(childModules))

	rootModule := runtimeGraph.Module("")
	rootChildren, err := moduleResolver.ChildModules(context.Background(), rootModule)
	require.NoError(t, err)
	require.Equal(t, []string{childModuleID}, moduleIDs(rootChildren))

	childModule := runtimeGraph.Module(childModuleID)
	moduleParent, err := moduleResolver.ParentModule(context.Background(), childModule)
	require.NoError(t, err)
	require.Equal(t, "", moduleParent.ID)

	createdBy, err := moduleResolver.CreatedBy(context.Background(), childModule)
	require.NoError(t, err)
	require.Equal(t, ownerID, createdBy.ID)

	components, err := moduleResolver.Components(context.Background(), childModule)
	require.NoError(t, err)
	require.Equal(t, []string{childID}, componentIDs(components))
}

func TestComponentQueriesUseCompleteRuntimeGraph(t *testing.T) {
	const (
		ownerID       = "custom.pipeline.default"
		childModuleID = ownerID
		sourceID      = childModuleID + "/prometheus.scrape.default"
		destinationID = childModuleID + "/prometheus.remote_write.default"
	)
	owner := &component.Info{
		ID:            component.ParseID(ownerID),
		ComponentName: "custom.pipeline",
		ModuleIDs:     []string{childModuleID},
	}
	source := &component.Info{
		ID:              component.ParseID(sourceID),
		ComponentName:   "prometheus.scrape",
		DataFlowEdgesTo: []string{"prometheus.remote_write.default"},
	}
	destination := &component.Info{
		ID:            component.ParseID(destinationID),
		ComponentName: "prometheus.remote_write",
	}
	resolver := &Resolver{Host: newResolverHost(owner, source, destination)}
	queryResolver := &queryResolver{Resolver: resolver}

	components, err := queryResolver.Components(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, components, 3)
	require.Empty(t, componentByID(components, ownerID).Edges.Outgoing)
	require.Equal(t, sourceID, componentByID(components, destinationID).Edges.Incoming[0].ComponentID)

	component, err := queryResolver.Component(context.Background(), destinationID)
	require.NoError(t, err)
	require.Equal(t, sourceID, component.Edges.Incoming[0].ComponentID)

	modules, err := queryResolver.Modules(context.Background())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"", childModuleID}, moduleIDs(modules))

	module, err := queryResolver.Module(context.Background(), childModuleID)
	require.NoError(t, err)
	moduleComponents, err := (&moduleResolver{}).Components(context.Background(), module)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{sourceID, destinationID}, componentIDs(moduleComponents))

	edgeComponent, err := (&componentEdgeResolver{Resolver: resolver}).Component(
		context.Background(),
		&componentByID(components, sourceID).Edges.Outgoing[0],
	)
	require.NoError(t, err)
	require.Equal(t, destinationID, edgeComponent.ID)
	require.Equal(t, sourceID, edgeComponent.Edges.Incoming[0].ComponentID)
}

func componentByID(components []model.Component, id string) *model.Component {
	for i := range components {
		if components[i].ID == id {
			return &components[i]
		}
	}
	return nil
}

func moduleIDs(modules []model.Module) []string {
	ids := make([]string, len(modules))
	for i, module := range modules {
		ids[i] = module.ID
	}
	return ids
}

func componentIDs(components []model.Component) []string {
	ids := make([]string, len(components))
	for i, component := range components {
		ids[i] = component.ID
	}
	return ids
}

type resolverHost struct {
	components map[string]*component.Info
}

func newResolverHost(components ...*component.Info) *resolverHost {
	host := &resolverHost{components: make(map[string]*component.Info, len(components))}
	for _, info := range components {
		host.components[info.ID.String()] = info
	}
	return host
}

func (h *resolverHost) GetComponent(id component.ID, _ component.InfoOptions) (*component.Info, error) {
	info, found := h.components[id.String()]
	if !found {
		return nil, component.ErrComponentNotFound
	}
	return info, nil
}

func (h *resolverHost) ListComponents(moduleID string, _ component.InfoOptions) ([]*component.Info, error) {
	var components []*component.Info
	for _, info := range h.components {
		if info.ID.ModuleID == moduleID {
			components = append(components, info)
		}
	}
	if len(components) == 0 {
		return nil, component.ErrModuleNotFound
	}
	return components, nil
}

func (h *resolverHost) GetService(string) (service.Service, bool) { return nil, false }

func (h *resolverHost) GetServiceConsumers(string) []service.Consumer { return nil }

func (h *resolverHost) NewController(string) (service.Controller, error) {
	return nil, errors.New("not implemented")
}

type recordingRateSnapshotter struct {
	windowSeconds int
}

func (s *recordingRateSnapshotter) RateSnapshot(windowSeconds int) ([]livedebugging.Data, error) {
	s.windowSeconds = windowSeconds
	return nil, nil
}

func (s *recordingRateSnapshotter) AddCallback(service.Host, livedebugging.CallbackID, livedebugging.ComponentID, func(livedebugging.Data)) error {
	return nil
}

func (s *recordingRateSnapshotter) DeleteCallback(livedebugging.CallbackID, livedebugging.ComponentID) {
}

func (s *recordingRateSnapshotter) AddCallbackMulti(service.Host, livedebugging.CallbackID, livedebugging.ModuleID, func(livedebugging.Data)) error {
	return nil
}

func (s *recordingRateSnapshotter) DeleteCallbackMulti(service.Host, livedebugging.CallbackID, livedebugging.ModuleID) {
}
