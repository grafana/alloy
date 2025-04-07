package controller

import (
	"fmt"
	"strings"
	"sync"

	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
	"go.opentelemetry.io/otel/trace"
)

type TracingConfigNode struct {
	nodeID        string
	componentName string
	traceProvider trace.TracerProvider // Tracer shared between all managed components.

	mut   sync.RWMutex
	block *ast.BlockStmt // Current Alloy blocks to derive config from
	eval  *vm.Evaluator
}

var _ BlockNode = (*TracingConfigNode)(nil)

// NewTracingConfigNode creates a new TracingConfigNode from an initial ast.BlockStmt.
// The underlying config isn't applied until Evaluate is called.
func NewTracingConfigNode(block *ast.BlockStmt, globals ComponentGlobals) *TracingConfigNode {
	return &TracingConfigNode{
		nodeID:        BlockComponentID(block).String(),
		componentName: block.GetBlockName(),
		traceProvider: globals.TraceProvider,

		block: block,
		eval:  vm.New(block.Body),
	}
}

// NewDefaulTracingConfigNode creates a new TracingConfigNode with nil block and eval.
// This will force evaluate to use the default tracing options for this node.
func NewDefaulTracingConfigNode(globals ComponentGlobals) *TracingConfigNode {
	return &TracingConfigNode{
		nodeID:        tracingBlockID,
		componentName: tracingBlockID,
		traceProvider: globals.TraceProvider,

		block: nil,
		eval:  nil,
	}
}

// Evaluate implements BlockNode and updates the arguments for the managed config block
// by re-evaluating its Alloy block with the provided scope. The managed config block
// will be built the first time Evaluate is called.
//
// Evaluate will return an error if the Alloy block cannot be evaluated or if
// decoding to arguments fails.
func (cn *TracingConfigNode) Evaluate(scope *vm.Scope) error {
	return cn.evaluate(scope, false)
}

func (cn *TracingConfigNode) TypeCheck(scope *vm.Scope) error {
	return cn.evaluate(scope, true)
}

func (cn *TracingConfigNode) evaluate(scope *vm.Scope, typeCheck bool) error {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	args := tracing.DefaultOptions
	if cn.eval != nil {
		if err := cn.eval.Evaluate(scope, &args); err != nil {
			return fmt.Errorf("decoding configuration: %w", err)
		}
	}

	if typeCheck {
		return nil
	}

	t, ok := cn.traceProvider.(*tracing.Tracer)
	if ok {
		err := t.Update(args)
		if err != nil {
			return fmt.Errorf("could not update tracer: %v", err)
		}
	}

	return nil
}

// Block implements BlockNode and returns the current block of the managed config node.
func (cn *TracingConfigNode) Block() *ast.BlockStmt {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.block
}

// NodeID implements dag.Node and returns the unique ID for the config node.
func (cn *TracingConfigNode) NodeID() string { return cn.nodeID }

// UpdateBlock updates the Alloy block used to construct arguments.
// The new block isn't used until the next time Evaluate is invoked.
//
// UpdateBlock will panic if the block does not match the component ID of the
// LoggingConfigNode.
func (cn *TracingConfigNode) UpdateBlock(b *ast.BlockStmt) {
	if !BlockComponentID(b).Equals(strings.Split(cn.nodeID, ".")) {
		panic("UpdateBlock called with an Alloy block with a different ID")
	}

	cn.mut.Lock()
	defer cn.mut.Unlock()
	cn.block = b
	cn.eval = vm.New(b.Body)
}
