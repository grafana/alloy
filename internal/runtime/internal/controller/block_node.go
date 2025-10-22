package controller

import (
	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

// BlockNode is a node in the DAG which manages an Alloy block
// and can be evaluated.
type BlockNode interface {
	dag.Node

	// Block returns the current block managed by the node.
	Block() *ast.BlockStmt

	// Evaluate updates the arguments by re-evaluating the Alloy block with the provided scope.
	//
	// Evaluate will return an error if the Alloy block cannot be evaluated or if
	// decoding to arguments fails.
	Evaluate(scope *vm.Scope) error

	// UpdateBlock updates the Alloy block used to construct arguments.
	UpdateBlock(b *ast.BlockStmt)
}
