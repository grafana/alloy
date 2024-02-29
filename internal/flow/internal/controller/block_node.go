package controller

import (
	"github.com/grafana/agent/internal/flow/internal/dag"
	"github.com/grafana/river/ast"
	"github.com/grafana/river/vm"
)

// BlockNode is a node in the DAG which manages a River block
// and can be evaluated.
type BlockNode interface {
	dag.Node

	// Block returns the current block of the managed config node.
	Block() *ast.BlockStmt

	// Evaluate updates the arguments for the managed component
	// by re-evaluating its River block with the provided scope. The managed component
	// will be built the first time Evaluate is called.
	//
	// Evaluate will return an error if the River block cannot be evaluated or if
	// decoding to arguments fails.
	Evaluate(scope *vm.Scope) error
}
