package runtime

import (
	"errors"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
)

const remoteConfigServiceName = "remotecfg"

type remoteConfigData interface {
	GetHost() service.Host
}

type componentListResult struct {
	// components is populated when the host can list the requested module.
	components []*component.Info

	// missingModule means the host was reachable, but it does not know about the
	// requested module. Callers use this to decide whether another host can still
	// satisfy the request.
	missingModule bool

	// unavailable means the host itself cannot currently be queried. This is
	// only used for the remote-config host, which is optional while Alloy starts.
	unavailable bool
}

// ListComponents returns a unified component list from the local Alloy host and
// the optional remote-config host.
//
// Remote-config components are exposed as part of the same GraphQL component
// graph. A moduleID prefixed with "remotecfg" is routed only to the remote host;
// other moduleIDs are queried against both hosts so local and remote components
// can be listed together.
func ListComponents(host service.Host, moduleID string, opts component.InfoOptions) ([]*component.Info, error) {
	local, err := listLocalComponents(host, moduleID, opts)
	if err != nil {
		return nil, err
	}

	remote, err := listRemoteComponents(host, moduleID, opts)
	if err != nil {
		return nil, err
	}

	if shouldReturnEmptyRemoteModule(moduleID, remote) {
		return nil, nil
	}
	if moduleMissing(local, remote) {
		return nil, component.ErrModuleNotFound
	}

	return append(local.components, remote.components...), nil
}

// ListAllComponents returns components from the root graph and every module
// created by root or nested components.
func ListAllComponents(host service.Host, opts component.InfoOptions) ([]*component.Info, error) {
	components, err := ListComponents(host, "", opts)
	if err != nil {
		return nil, err
	}

	result := make([]*component.Info, 0, len(components))
	seenComponents := make(map[string]struct{}, len(components))
	seenModules := map[string]struct{}{"": {}}
	var modules []string

	appendComponents := func(infos []*component.Info) {
		for _, info := range infos {
			if _, seen := seenComponents[info.ID.String()]; seen {
				continue
			}
			seenComponents[info.ID.String()] = struct{}{}
			result = append(result, info)
			modules = append(modules, info.ModuleIDs...)
		}
	}
	appendComponents(components)

	for len(modules) > 0 {
		moduleID := modules[0]
		modules = modules[1:]
		if _, seen := seenModules[moduleID]; seen {
			continue
		}
		seenModules[moduleID] = struct{}{}

		components, err := ListComponents(host, moduleID, opts)
		if err != nil {
			return nil, err
		}
		appendComponents(components)
	}

	return result, nil
}

// listLocalComponents lists components from the main Alloy host.
//
// The local host does not own the synthetic "remotecfg" module namespace used by
// GraphQL, so those requests are reported as a missing local module and left for
// the remote-config host to satisfy.
func listLocalComponents(host service.Host, moduleID string, opts component.InfoOptions) (componentListResult, error) {
	if isRemoteConfigModule(moduleID) {
		return componentListResult{missingModule: true}, nil
	}
	return listComponents(host, moduleID, opts)
}

// listRemoteComponents lists components from the remote-config host when it is
// available.
//
// Missing or not-yet-started remote config is not a hard error for GraphQL. The
// caller decides whether an unavailable remote host should produce an empty list
// or a module-not-found error based on the requested moduleID.
func listRemoteComponents(host service.Host, moduleID string, opts component.InfoOptions) (componentListResult, error) {
	remoteHost, err := remoteConfigHost(host)
	if err != nil {
		return componentListResult{unavailable: true}, nil
	}

	return listComponents(remoteHost, remoteModuleID(moduleID), opts)
}

// listComponents normalizes service.Host.ListComponents into componentListResult.
//
// ErrModuleNotFound is represented as data instead of an error so ListComponents
// can merge results across local and remote hosts before deciding what to return.
func listComponents(host service.Host, moduleID string, opts component.InfoOptions) (componentListResult, error) {
	components, err := host.ListComponents(moduleID, opts)
	if errors.Is(err, component.ErrModuleNotFound) {
		return componentListResult{missingModule: true}, nil
	}
	if err != nil {
		return componentListResult{}, err
	}
	return componentListResult{components: components}, nil
}

// shouldReturnEmptyRemoteModule reports whether a remote-config module query
// should produce an empty list because the optional remote host is unavailable.
func shouldReturnEmptyRemoteModule(moduleID string, remote componentListResult) bool {
	return isRemoteConfigModule(moduleID) && remote.unavailable
}

// moduleMissing reports whether neither host can satisfy the requested module.
func moduleMissing(local, remote componentListResult) bool {
	return local.missingModule && (remote.missingModule || remote.unavailable)
}

// isRemoteConfigModule reports whether moduleID is in GraphQL's synthetic
// remote-config namespace.
func isRemoteConfigModule(moduleID string) bool {
	return moduleID == "remotecfg" || strings.HasPrefix(moduleID, "remotecfg/")
}

// remoteModuleID maps GraphQL's synthetic remote-config namespace back to the
// module IDs used by the remote-config host.
func remoteModuleID(moduleID string) string {
	if moduleID == "remotecfg" {
		return ""
	}
	return moduleID
}

// LoadGraph returns one consistent snapshot of all components and modules.
func LoadGraph(host service.Host, opts component.InfoOptions) (*model.RuntimeGraph, error) {
	infos, err := ListAllComponents(host, opts)
	if err != nil {
		return nil, err
	}
	return NewGraph(infos), nil
}

// NewGraph builds the component data-flow graph and module hierarchy.
func NewGraph(infos []*component.Info) *model.RuntimeGraph {
	components := NewComponents(infos)
	modules := make([]model.Module, 0)
	moduleIndexes := make(map[string]int)

	ensureModule := func(id string) int {
		if index, found := moduleIndexes[id]; found {
			return index
		}
		moduleIndexes[id] = len(modules)
		modules = append(modules, model.Module{ID: id})
		return len(modules) - 1
	}

	for _, info := range infos {
		moduleIndex := ensureModule(info.ID.ModuleID)
		modules[moduleIndex].ComponentIDs = appendUnique(modules[moduleIndex].ComponentIDs, info.ID.String())

		for _, childModuleID := range info.ModuleIDs {
			childModuleIndex := ensureModule(childModuleID)
			parentModuleID := info.ID.ModuleID
			createdByComponentID := info.ID.String()
			modules[childModuleIndex].ParentModuleID = &parentModuleID
			modules[childModuleIndex].CreatedByComponentID = &createdByComponentID
			modules[moduleIndex].ChildModuleIDs = appendUnique(modules[moduleIndex].ChildModuleIDs, childModuleID)
		}
	}

	return model.NewRuntimeGraph(components, modules)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// NewComponents converts component.Info values into GraphQL component models and
// fills each component's incoming edges from other components' outgoing
// DataFlowEdgesTo entries.
func NewComponents(infos []*component.Info) []model.Component {
	components := make([]model.Component, len(infos))
	incomingEdges := incomingDataFlowEdges(infos)

	for i, info := range infos {
		components[i] = model.NewComponent(info)
		componentID := info.ID.String()
		components[i].Edges.Incoming = incomingEdges[componentID]
	}
	return components
}

// incomingDataFlowEdges builds the reverse edge index for DataFlowEdgesTo.
func incomingDataFlowEdges(infos []*component.Info) map[string][]model.ComponentEdge {
	incoming := map[string][]model.ComponentEdge{}
	for _, info := range infos {
		sourceID := info.ID.String()
		for _, edge := range model.NewComponentEdges(info).Outgoing {
			incoming[edge.ComponentID] = append(incoming[edge.ComponentID], edgeFromSource(sourceID))
		}
	}
	return incoming
}

// edgeFromSource creates an incoming edge pointing back to sourceID.
func edgeFromSource(sourceID string) model.ComponentEdge {
	return model.ComponentEdge{
		SourceID:    sourceID,
		ComponentID: sourceID,
	}
}

// remoteConfigHost returns the service host used by remote configuration.
func remoteConfigHost(host service.Host) (service.Host, error) {
	svc, found := host.GetService(remoteConfigServiceName)
	if !found {
		return nil, errors.New("remote config service not available")
	}

	data, ok := svc.Data().(remoteConfigData)
	if !ok || data.GetHost() == nil {
		return nil, errors.New("remote config service startup in progress")
	}
	return data.GetHost(), nil
}
