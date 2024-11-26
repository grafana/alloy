package controller

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/vm"
)

// This special keyword is used to expose the argument values to the custom components.
const argumentLabel = "argument"

// valueCache caches exports to expose as variables for Alloy expressions.
// The exports are stored directly in the scope which is used to evaluate Alloy expressions.
type valueCache struct {
	mut                sync.RWMutex
	componentIds       map[string]ComponentID
	moduleExports      map[string]any // name -> value for the value of module exports
	moduleChangedIndex int            // Everytime a change occurs this is incremented
	scope              *vm.Scope      // scope provides additional context for the nodes in the module
}

// newValueCache creates a new ValueCache.
func newValueCache() *valueCache {
	return &valueCache{
		componentIds:  make(map[string]ComponentID, 0),
		moduleExports: make(map[string]any),
		scope:         vm.NewScope(make(map[string]any)),
	}
}

// UpdateScopeVariables updates the Variables map of the scope with a deep copy of the provided map.
func (vc *valueCache) UpdateScopeVariables(variables map[string]any) {
	if variables == nil {
		return
	}
	vc.mut.Lock()
	defer vc.mut.Unlock()
	vc.scope.Variables = deepCopyMap(variables)
}

// CacheExports will cache the provided exports using the given id. exports may
// be nil to store an empty object.
func (vc *valueCache) CacheExports(id ComponentID, exports component.Exports) error {
	vc.mut.Lock()
	defer vc.mut.Unlock()

	variables := vc.scope.Variables
	// Build nested maps.
	for i := 0; i < len(id)-1; i++ {
		t := id[i]
		if _, ok := variables[t]; !ok {
			variables[t] = make(map[string]any)
		} else if _, ok := variables[t].(map[string]any); !ok {
			return fmt.Errorf("expected a map but found a value for %q when trying to cache the export for %s", t, id.String())
		}
		variables = variables[t].(map[string]any)
	}

	var exportsVal any = make(map[string]any)
	if exports != nil {
		exportsVal = exports
	}
	variables[id[len(id)-1]] = exportsVal
	return nil
}

func (vc *valueCache) GetModuleArgument(key string) (any, bool) {
	vc.mut.RLock()
	defer vc.mut.RUnlock()
	if _, ok := vc.scope.Variables[argumentLabel]; !ok {
		return nil, false
	}
	v, exist := vc.scope.Variables[argumentLabel].(map[string]any)[key]
	return v, exist
}

// CacheModuleArgument will cache the provided exports using the given id.
func (vc *valueCache) CacheModuleArgument(key string, value any) {
	vc.mut.Lock()
	defer vc.mut.Unlock()

	if _, ok := vc.scope.Variables[argumentLabel]; !ok {
		vc.scope.Variables[argumentLabel] = make(map[string]any)
	}
	keyMap := make(map[string]any)
	keyMap["value"] = value
	vc.scope.Variables[argumentLabel].(map[string]any)[key] = keyMap
}

// CacheModuleExportValue saves the value to the map
func (vc *valueCache) CacheModuleExportValue(name string, value any) {
	vc.mut.Lock()
	defer vc.mut.Unlock()

	// Need to see if the module exports have changed.
	v, found := vc.moduleExports[name]
	if !found {
		vc.moduleChangedIndex++
	} else if !reflect.DeepEqual(v, value) {
		vc.moduleChangedIndex++
	}

	vc.moduleExports[name] = value
}

// CreateModuleExports creates a map for usage on OnExportsChanged
func (vc *valueCache) CreateModuleExports() map[string]any {
	vc.mut.RLock()
	defer vc.mut.RUnlock()

	exports := make(map[string]any)
	for k, v := range vc.moduleExports {
		exports[k] = v
	}
	return exports
}

// ClearModuleExports empties the map and notifies that the exports have changed.
func (vc *valueCache) ClearModuleExports() {
	vc.mut.Lock()
	defer vc.mut.Unlock()

	vc.moduleChangedIndex++
	vc.moduleExports = make(map[string]any)
}

// ExportChangeIndex return the change index.
func (vc *valueCache) ExportChangeIndex() int {
	vc.mut.RLock()
	defer vc.mut.RUnlock()

	return vc.moduleChangedIndex
}

// SyncIDs will remove any cached values for any Component ID from the graph which is not in ids.
// SyncIDs should be called with the current set of components after the graph is updated.
func (vc *valueCache) SyncIDs(ids map[string]ComponentID) error {
	vc.mut.Lock()
	defer vc.mut.Unlock()

	// Find the components that should be removed.
	cleanupIds := make([]ComponentID, 0)
	for name, id := range vc.componentIds {
		if _, exist := ids[name]; !exist {
			cleanupIds = append(cleanupIds, id)
		}
	}

	// Remove the component exports from the scope.
	for _, id := range cleanupIds {
		err := cleanup(vc.scope.Variables, id, 0)
		if err != nil {
			return fmt.Errorf("failed to sync component %s: %w", id.String(), err)
		}
	}
	vc.componentIds = ids
	return nil
}

// cleanup removes the ComponentID path from the map
func cleanup(m map[string]any, id ComponentID, index int) error {

	if _, ok := m[id[index]]; !ok {
		return nil
	}

	if index == len(id)-1 {
		delete(m, id[index]) // Remove the component's exports.
		return nil
	}

	if _, ok := m[id[index]].(map[string]any); !ok {
		return fmt.Errorf("expected a map but found a value for %q", id[index])
	}
	nextM := m[id[index]].(map[string]any)

	err := cleanup(nextM, id, index+1)
	if err != nil {
		return err
	}

	// Delete if the map at this level is empty.
	// If you only have one Prometheus component and you remove it, it will cleanup the full Prometheus path.
	// If you have one Prometheus relabel and one Prometheus scrape, and you remove the relabel, it will cleanup the relabel path.
	if len(nextM) == 0 {
		delete(m, id[index])
	}
	return nil
}

// SyncModuleArgs will remove any cached values for any args no longer in the map.
func (vc *valueCache) SyncModuleArgs(args map[string]any) {
	vc.mut.Lock()
	defer vc.mut.Unlock()

	if _, ok := vc.scope.Variables[argumentLabel]; !ok {
		return
	}

	argsMap := vc.scope.Variables[argumentLabel].(map[string]any)
	for arg := range argsMap {
		if _, ok := args[arg]; !ok {
			delete(argsMap, arg)
		}
	}
	if len(argsMap) == 0 {
		delete(vc.scope.Variables, argumentLabel)
	}
}

// GetScope returns the current scope.
func (vc *valueCache) GetScope() *vm.Scope {
	vc.mut.RLock()
	defer vc.mut.RUnlock()
	return vm.NewScope(deepCopyMap(vc.scope.Variables))
}

func deepCopyMap(original map[string]any) map[string]any {
	newMap := make(map[string]any, len(original))
	for key, value := range original {
		switch v := value.(type) {
		case map[string]any:
			newMap[key] = deepCopyMap(v)
		default:
			newMap[key] = v
		}
	}
	return newMap
}
