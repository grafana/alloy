package controller

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

// ServiceNode is a DAG node which represents a running service.
type ServiceNode struct {
	host service.Host
	svc  service.Service
	def  service.Definition

	mut   sync.RWMutex
	block *ast.BlockStmt // Current Alloy block to derive args from
	eval  *vm.Evaluator
	args  component.Arguments // Evaluated arguments for the managed component

	exports component.Exports // Evaluated exports for the managed component
}

var _ RunnableNode = (*ServiceNode)(nil)

// NewServiceNode creates a new instance of a ServiceNode from an instance of a
// Service. The provided host is used when running the service.
func NewServiceNode(host service.Host, svc service.Service) *ServiceNode {
	def := svc.Definition()
	s := &ServiceNode{
		host:    host,
		svc:     svc,
		def:     def,
		exports: svc.Exports(),
	}
	return s
}

// Service returns the service instance associated with the node.
func (sn *ServiceNode) Service() service.Service { return sn.svc }

// Definition returns the service definition associated with the node.
func (sn *ServiceNode) Definition() service.Definition { return sn.def }

// NodeID returns the ID of the ServiceNode, which is equal to the service's
// name.
func (sn *ServiceNode) NodeID() string { return sn.def.Name }

// Block implements BlockNode. It returns nil, since ServiceNodes don't have
// associated configs.
func (sn *ServiceNode) Block() *ast.BlockStmt {
	sn.mut.RLock()
	defer sn.mut.RUnlock()
	return sn.block
}

// UpdateBlock updates the Alloy block used to construct arguments for the
// service. The new block isn't used until the next time Evaluate is called.
//
// UpdateBlock will panic if the block does not match the ID of the
// ServiceNode.
//
// Call UpdateBlock with a nil block to remove the block associated with the
// ServiceNode.
func (sn *ServiceNode) UpdateBlock(b *ast.BlockStmt) {
	if b != nil && !BlockComponentID(b).Equals([]string{sn.NodeID()}) {
		panic("UpdateBlock called with a block with a different block ID")
	}

	sn.mut.Lock()
	defer sn.mut.Unlock()

	sn.block = b

	if b != nil {
		sn.eval = vm.New(b.Body)
	} else {
		sn.eval = vm.New(ast.Body{})
	}
}

// Evaluate implements BlockNode, evaluating the configuration for a service.
// Evalute returns an error if the service doesn't support being configured and
// the ServiceNode has an associated block from a call to UpdateBlock.
func (sn *ServiceNode) Evaluate(scope *vm.Scope) error {
	sn.mut.Lock()
	defer sn.mut.Unlock()

	switch {
	case sn.block != nil && sn.def.ConfigType == nil:
		return fmt.Errorf("service %q does not support being configured", sn.NodeID())

	case sn.def.ConfigType == nil:
		return nil // Do nothing; no configuration.
	}

	argsPointer := sn.def.CloneConfig()

	if err := sn.eval.Evaluate(scope, argsPointer); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	// args is always a pointer to the args type, so we want to deference it
	// since services expect a non-pointer.
	argsCopyValue := reflect.ValueOf(argsPointer).Elem().Interface()

	if equality.DeepEqual(sn.args, argsCopyValue) {
		// Ignore arguments which haven't changed. This reduces the cost of calling
		// evaluate for services where evaluation is expensive (e.g., if
		// re-evaluating requires re-starting some internal logic).
		return nil
	}

	// Update the service.
	if err := sn.svc.Update(argsCopyValue); err != nil {
		return fmt.Errorf("updating service: %w", err)
	}

	sn.args = argsCopyValue

	// Update exports to the service node
	var expectedExportsType reflect.Type
	if sn.def.ExportsType != nil {
		expectedExportsType = reflect.TypeOf(sn.def.ExportsType)
	}

	exp := sn.svc.Exports()

	if expectedExportsType != reflect.TypeOf(exp) {
		return fmt.Errorf("service %s changed Exports types from %T to %T", sn.NodeID(), sn.exports, exp)
	}

	sn.exports = exp
	return nil
}

func (sn *ServiceNode) Run(ctx context.Context) error {
	return sn.svc.Run(ctx, sn.host)
}

func (sn *ServiceNode) Exports() component.Exports {
	sn.mut.RLock()
	defer sn.mut.RUnlock()
	return sn.exports
}
