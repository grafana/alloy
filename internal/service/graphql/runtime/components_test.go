package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	graphqlruntime "github.com/grafana/alloy/internal/service/graphql/runtime"
	"github.com/grafana/alloy/internal/service/remotecfg"
)

const (
	prometheusScrapeName  = "prometheus.scrape"
	rootScrapeID          = "prometheus.scrape.root"
	remoteScrapeID        = "prometheus.scrape.remote"
	remoteModuleScrapeID  = "module_a/prometheus.scrape.remote_module"
	remoteWriteDefaultID  = "prometheus.remote_write.default"
	blackboxExporterID    = "prometheus.exporter.blackbox.local"
	blackboxProbeScrapeID = "prometheus.scrape.blackbox_probe"
)

func TestListComponentsIncludesRootAndRemoteComponents(t *testing.T) {
	root := newFakeHost(componentInfo(rootScrapeID, prometheusScrapeName, "root"))
	remote := newFakeHost(componentInfo(remoteScrapeID, prometheusScrapeName, "remote"))
	root.services[remotecfg.ServiceName] = fakeRemoteConfigService{host: remote}

	components, err := graphqlruntime.ListComponents(root, "", component.InfoOptions{GetHealth: true})

	require.NoError(t, err)
	require.Len(t, components, 2)
	require.Equal(t, rootScrapeID, components[0].ID.String())
	require.Equal(t, remoteScrapeID, components[1].ID.String())
}

func TestListComponentsFiltersByModuleID(t *testing.T) {
	root := newFakeHost(
		componentInfo(rootScrapeID, prometheusScrapeName, "root"),
		componentInfo("module_a/prometheus.scrape.module", prometheusScrapeName, "module"),
	)
	remote := newFakeHost(
		componentInfo(remoteScrapeID, prometheusScrapeName, "remote"),
		componentInfo(remoteModuleScrapeID, prometheusScrapeName, "remote-module"),
	)
	root.services[remotecfg.ServiceName] = fakeRemoteConfigService{host: remote}

	components, err := graphqlruntime.ListComponents(root, "module_a", component.InfoOptions{GetHealth: true})

	require.NoError(t, err)
	require.Len(t, components, 2)
	require.Equal(t, "module_a/prometheus.scrape.module", components[0].ID.String())
	require.Equal(t, remoteModuleScrapeID, components[1].ID.String())
}

func TestListComponentsTreatsModuleMissingInOneHostAsEmpty(t *testing.T) {
	root := newFakeHost(componentInfo(rootScrapeID, prometheusScrapeName, "root"))
	remote := newFakeHost(componentInfo(remoteModuleScrapeID, prometheusScrapeName, "remote-module"))
	root.services[remotecfg.ServiceName] = fakeRemoteConfigService{host: remote}
	root.missingModules = map[string]bool{"module_a": true}

	components, err := graphqlruntime.ListComponents(root, "module_a", component.InfoOptions{GetHealth: true})

	require.NoError(t, err)
	require.Len(t, components, 1)
	require.Equal(t, remoteModuleScrapeID, components[0].ID.String())
}

func TestListComponentsTreatsUnavailableRemoteConfigAsEmpty(t *testing.T) {
	root := newFakeHost()
	root.missingModules = map[string]bool{"remotecfg": true}

	components, err := graphqlruntime.ListComponents(root, "remotecfg", component.InfoOptions{GetHealth: true})

	require.NoError(t, err)
	require.Empty(t, components)
}

func TestListComponentsPreservesRemoteChildModulePrefix(t *testing.T) {
	root := newFakeHost()
	remote := newFakeHost(componentInfo(
		"remotecfg/module_a/prometheus.scrape.remote",
		prometheusScrapeName,
		"remote",
	))
	root.services[remotecfg.ServiceName] = fakeRemoteConfigService{host: remote}

	components, err := graphqlruntime.ListComponents(root, "remotecfg/module_a", component.InfoOptions{GetHealth: true})

	require.NoError(t, err)
	require.Len(t, components, 1)
	require.Equal(t, []string{"remotecfg/module_a"}, remote.listCalls)
}

func TestListAllComponentsIncludesCreatedModules(t *testing.T) {
	rootComponent := componentInfo("custom.pipeline.default", "custom.pipeline", "default")
	rootComponent.ModuleIDs = []string{"custom.pipeline.default"}
	childComponent := componentInfo(
		"custom.pipeline.default/prometheus.scrape.default",
		prometheusScrapeName,
		"default",
	)
	root := newFakeHost(rootComponent, childComponent)

	components, err := graphqlruntime.ListAllComponents(root, component.InfoOptions{GetHealth: true})

	require.NoError(t, err)
	require.Len(t, components, 2)
	require.ElementsMatch(t, []string{"", "custom.pipeline.default"}, root.listCalls)
}

func TestNewComponentsAddsIncomingDataFlowEdges(t *testing.T) {
	source := componentInfo("prometheus.scrape.default", prometheusScrapeName, "default")
	source.DataFlowEdgesTo = []string{remoteWriteDefaultID}
	destination := componentInfo(remoteWriteDefaultID, "prometheus.remote_write", "default")

	components := graphqlruntime.NewComponents([]*component.Info{source, destination})

	require.Len(t, components[1].Edges.Incoming, 1)
	require.Equal(t, "prometheus.scrape.default", components[1].Edges.Incoming[0].ComponentID)
}

func TestNewComponentsQualifiesNestedDataFlowEdges(t *testing.T) {
	const moduleID = "remotecfg/module_a"
	source := componentInfo(moduleID+"/prometheus.scrape.default", prometheusScrapeName, "default")
	source.DataFlowEdgesTo = []string{remoteWriteDefaultID}
	destination := componentInfo(moduleID+"/"+remoteWriteDefaultID, "prometheus.remote_write", "default")

	components := graphqlruntime.NewComponents([]*component.Info{source, destination})

	require.Equal(t, moduleID+"/"+remoteWriteDefaultID, components[0].Edges.Outgoing[0].ComponentID)
	require.Equal(t, source.ID.String(), components[1].Edges.Incoming[0].ComponentID)
}

func TestNewComponentsKeepsModuleContainmentOutOfDataFlowEdges(t *testing.T) {
	const moduleID = "remotecfg/my_target_test_thing.default"
	owner := componentInfo("remotecfg/my_target_test_thing.default", "my_target_test_thing", "default")
	owner.ModuleIDs = []string{moduleID}
	entry := componentInfo(moduleID+"/discovery.file.mydiscovery", "discovery.file", "mydiscovery")
	destination := componentInfo(moduleID+"/"+prometheusScrapeName+".mydiscovery", prometheusScrapeName, "mydiscovery")
	entry.DataFlowEdgesTo = []string{prometheusScrapeName + ".mydiscovery"}

	components := graphqlruntime.NewComponents([]*component.Info{owner, entry, destination})

	require.Empty(t, components[0].Edges.Outgoing)
	require.Empty(t, components[1].Edges.Incoming)
}

func TestNewGraphBuildsNestedModuleHierarchy(t *testing.T) {
	const (
		ownerID       = "custom.pipeline.default"
		childModuleID = ownerID
		nestedOwnerID = childModuleID + "/foreach.default"
		grandchildID  = nestedOwnerID + "/0"
		leafID        = grandchildID + "/prometheus.scrape.default"
	)

	owner := componentInfo(ownerID, "custom.pipeline", "default")
	owner.ModuleIDs = []string{childModuleID}
	nestedOwner := componentInfo(nestedOwnerID, "foreach", "default")
	nestedOwner.ModuleIDs = []string{grandchildID}
	leaf := componentInfo(leafID, prometheusScrapeName, "default")

	graph := graphqlruntime.NewGraph([]*component.Info{owner, nestedOwner, leaf})

	root := graph.Module("")
	require.NotNil(t, root)
	require.Nil(t, root.ParentModuleID)
	require.Nil(t, root.CreatedByComponentID)
	require.Equal(t, []string{ownerID}, root.ComponentIDs)
	require.Equal(t, []string{childModuleID}, root.ChildModuleIDs)

	child := graph.Module(childModuleID)
	require.NotNil(t, child)
	require.Equal(t, "", *child.ParentModuleID)
	require.Equal(t, ownerID, *child.CreatedByComponentID)
	require.Equal(t, []string{nestedOwnerID}, child.ComponentIDs)
	require.Equal(t, []string{grandchildID}, child.ChildModuleIDs)

	grandchild := graph.Module(grandchildID)
	require.NotNil(t, grandchild)
	require.Equal(t, childModuleID, *grandchild.ParentModuleID)
	require.Equal(t, nestedOwnerID, *grandchild.CreatedByComponentID)
	require.Equal(t, []string{leafID}, grandchild.ComponentIDs)
	require.Empty(t, grandchild.ChildModuleIDs)
}

func TestNewComponentsIgnoresReferences(t *testing.T) {
	exporter := componentInfo(blackboxExporterID, "prometheus.exporter.blackbox", "local")
	exporter.ReferencedBy = []string{blackboxProbeScrapeID}
	exporter.DataFlowEdgesTo = []string{blackboxProbeScrapeID}
	scrape := componentInfo(blackboxProbeScrapeID, prometheusScrapeName, "blackbox_probe")
	scrape.References = []string{blackboxExporterID}

	components := graphqlruntime.NewComponents([]*component.Info{exporter, scrape})

	require.Empty(t, components[0].Edges.Incoming)
	require.Empty(t, components[1].Edges.Outgoing)
	require.Equal(t, []model.ComponentEdge{{
		SourceID:    blackboxExporterID,
		ComponentID: blackboxExporterID,
	}}, components[1].Edges.Incoming)
}

func TestComponentModelIncludesIdentityHealthEdgesAndLiveDebugFlag(t *testing.T) {
	updated := time.Date(2026, 4, 24, 20, 0, 0, 0, time.UTC)
	info := componentInfo("module_a/prometheus.scrape.default", prometheusScrapeName, "default")
	info.Type = component.TypeBuiltin
	info.References = []string{remoteWriteDefaultID}
	info.ReferencedBy = []string{"prometheus.relabel.default"}
	info.DataFlowEdgesTo = []string{remoteWriteDefaultID}
	info.Health = component.Health{
		Health:     component.HealthTypeHealthy,
		Message:    "ready",
		UpdateTime: updated,
	}
	info.LiveDebuggingEnabled = true

	got := model.NewComponent(info)

	require.Equal(t, "module_a/prometheus.scrape.default", got.ID)
	require.Equal(t, "module_a", got.ModuleID)
	require.Equal(t, "prometheus.scrape", got.Name)
	require.NotNil(t, got.Label)
	require.Equal(t, "default", *got.Label)
	require.Equal(t, "builtin", got.Type)
	require.Equal(t, "healthy", got.Health.State)
	require.Equal(t, "ready", got.Health.Message)
	require.Equal(t, updated, got.Health.LastUpdated)
	require.True(t, got.LiveDebuggingEnabled)
	require.Len(t, got.Edges.Outgoing, 1)
	require.Empty(t, got.Edges.Incoming)
}

type fakeHost struct {
	components     map[string]*component.Info
	services       map[string]service.Service
	missingModules map[string]bool
	listErr        error
	listCalls      []string
}

func newFakeHost(components ...*component.Info) *fakeHost {
	host := &fakeHost{
		components: make(map[string]*component.Info, len(components)),
		services:   map[string]service.Service{},
	}
	for _, info := range components {
		host.components[info.ID.String()] = info
	}
	return host
}

func (h *fakeHost) GetComponent(id component.ID, _ component.InfoOptions) (*component.Info, error) {
	info, ok := h.components[id.String()]
	if !ok {
		return nil, component.ErrComponentNotFound
	}
	return info, nil
}

func (h *fakeHost) ListComponents(moduleID string, _ component.InfoOptions) ([]*component.Info, error) {
	h.listCalls = append(h.listCalls, moduleID)
	if h.listErr != nil {
		return nil, h.listErr
	}
	if h.missingModules[moduleID] {
		return nil, component.ErrModuleNotFound
	}
	var components []*component.Info
	for _, info := range h.components {
		if info.ID.ModuleID == moduleID {
			components = append(components, info)
		}
	}
	return components, nil
}

func (h *fakeHost) GetService(name string) (service.Service, bool) {
	svc, ok := h.services[name]
	return svc, ok
}

func (h *fakeHost) GetServiceConsumers(string) []service.Consumer { return nil }

func (h *fakeHost) NewController(string) (service.Controller, error) {
	return nil, errors.New("not implemented")
}

type fakeRemoteConfigService struct {
	host service.Host
}

func (f fakeRemoteConfigService) Definition() service.Definition {
	return service.Definition{Name: remotecfg.ServiceName}
}

func (f fakeRemoteConfigService) Run(context.Context, service.Host) error { return nil }
func (f fakeRemoteConfigService) Update(any) error                        { return nil }
func (f fakeRemoteConfigService) Data() any                               { return remotecfg.Data{Host: f.host} }

func componentInfo(id string, name string, label string) *component.Info {
	return &component.Info{
		ID:            component.ParseID(id),
		ComponentName: name,
		Label:         label,
		Health: component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    "not ready",
			UpdateTime: time.Unix(0, 0).UTC(),
		},
	}
}
