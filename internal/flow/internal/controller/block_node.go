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

	// Block returns the current block managed by the node.
	Block() *ast.BlockStmt

	// Evaluate updates the arguments by re-evaluating the River block with the provided scope.
	//
	// Evaluate will return an error if the River block cannot be evaluated or if
	// decoding to arguments fails.
	Evaluate(scope *vm.Scope) error

	// UpdateBlock updates the River block used to construct arguments.
	UpdateBlock(b *ast.BlockStmt)
}
