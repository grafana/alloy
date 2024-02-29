package controller

import (
	"fmt"
	"sync"

	"github.com/grafana/agent/internal/flow/tracing"
	"github.com/grafana/river/ast"
	"github.com/grafana/river/vm"
	"go.opentelemetry.io/otel/trace"
)

type TracingConfigNode struct {
	nodeID        string
	componentName string
	traceProvider trace.TracerProvider // Tracer shared between all managed components.

	mut   sync.RWMutex
	block *ast.BlockStmt // Current River blocks to derive config from
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
// by re-evaluating its River block with the provided scope. The managed config block
// will be built the first time Evaluate is called.
//
// Evaluate will return an error if the River block cannot be evaluated or if
// decoding to arguments fails.
func (cn *TracingConfigNode) Evaluate(scope *vm.Scope) error {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	args := tracing.DefaultOptions
	if cn.eval != nil {
		if err := cn.eval.Evaluate(scope, &args); err != nil {
			return fmt.Errorf("decoding River: %w", err)
		}
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
