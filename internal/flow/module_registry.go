package flow

import (
	"fmt"
	"sync"
)

type moduleRegistry struct {
	mut     sync.RWMutex
	modules map[string]*module
}

func newModuleRegistry() *moduleRegistry {
	return &moduleRegistry{
		modules: make(map[string]*module),
	}
}

// Get retrieves a module by ID.
func (reg *moduleRegistry) Get(id string) (*module, bool) {
	reg.mut.RLock()
	defer reg.mut.RUnlock()

	mod, ok := reg.modules[id]
	return mod, ok
}

// List returns the set of all modules. The return order is not guaranteed.
func (reg *moduleRegistry) List() []*module {
	reg.mut.RLock()
	defer reg.mut.RUnlock()

	list := make([]*module, 0, len(reg.modules))
	for _, mod := range reg.modules {
		list = append(list, mod)
	}
	return list
}

// Register registers a module by ID. It returns an error if that module is
// already registered.
func (reg *moduleRegistry) Register(id string, mod *module) error {
	reg.mut.Lock()
	defer reg.mut.Unlock()

	if _, exist := reg.modules[id]; exist {
		return fmt.Errorf("module %q already exists", id)
	}

	reg.modules[id] = mod
	return nil
}

// Unregister unregisters a module by ID. It is a no-op if the provided ID
// isn't registered.
func (reg *moduleRegistry) Unregister(id string) {
	reg.mut.Lock()
	defer reg.mut.Unlock()

	delete(reg.modules, id)
}
