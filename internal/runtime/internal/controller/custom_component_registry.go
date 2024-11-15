package controller

import (
	"fmt"
	"sync"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

// CustomComponentRegistry holds custom component definitions that are available in the context.
// The definitions are either imported, declared locally, or declared in a parent registry.
// Imported definitions are stored inside of the corresponding import registry.
type CustomComponentRegistry struct {
	parent *CustomComponentRegistry // nil if root config

	mut      sync.RWMutex
	scope    *vm.Scope
	imports  map[string]*CustomComponentRegistry // importNamespace: importScope
	declares map[string]ast.Body                 // customComponentName: template
}

// NewCustomComponentRegistry creates a new CustomComponentRegistry with a parent.
// parent can be nil.
func NewCustomComponentRegistry(parent *CustomComponentRegistry, scope *vm.Scope) *CustomComponentRegistry {
	return &CustomComponentRegistry{
		parent:   parent,
		scope:    scope,
		declares: make(map[string]ast.Body),
		imports:  make(map[string]*CustomComponentRegistry),
	}
}

func (s *CustomComponentRegistry) getDeclare(name string) (ast.Body, bool) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	declare, ok := s.declares[name]
	return declare, ok
}

func (s *CustomComponentRegistry) getImport(name string) (*CustomComponentRegistry, bool) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	im, ok := s.imports[name]
	return im, ok
}

func (s *CustomComponentRegistry) Scope() *vm.Scope {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.scope
}

// registerDeclare stores a local declare block.
func (s *CustomComponentRegistry) registerDeclare(declare *ast.BlockStmt) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.declares[declare.Label] = declare.Body
}

// registerImport stores the import namespace.
// The content will be added later during evaluation.
// It's important to register it before populating the component nodes
// (else we don't know which one exists).
func (s *CustomComponentRegistry) registerImport(importNamespace string) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.imports[importNamespace] = nil
}

// updateImportContent updates the content of a registered import.
// The content of an import node can contain other import blocks.
// These are considered as "children" of the root import node.
// Each child has its own CustomComponentRegistry which needs to be updated.
func (s *CustomComponentRegistry) updateImportContent(importNode *ImportConfigNode) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if _, exist := s.imports[importNode.label]; !exist {
		panic(fmt.Errorf("import %q was not registered", importNode.label))
	}
	importScope := NewCustomComponentRegistry(nil, importNode.Scope())
	importScope.declares = importNode.ImportedDeclares()
	importScope.updateImportContentChildren(importNode)
	s.imports[importNode.label] = importScope
}

// updateImportContentChildren recurse through the children of an import node
// and update their scope with the imported declare blocks.
func (s *CustomComponentRegistry) updateImportContentChildren(importNode *ImportConfigNode) {
	for _, child := range importNode.ImportConfigNodesChildren() {
		childScope := NewCustomComponentRegistry(nil, child.Scope())
		childScope.declares = child.ImportedDeclares()
		childScope.updateImportContentChildren(child)
		s.imports[child.label] = childScope
	}
}
