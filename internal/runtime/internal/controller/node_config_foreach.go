package controller

import (
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

type ForeachConfigNode struct {
	nodeID string
	label  string
	block  *ast.BlockStmt // Current Alloy blocks to derive config from
}

var _ BlockNode = (*ForeachConfigNode)(nil)

// For now the Foreach doesn't have the ability to export arguments.
//TODO: We could implement this in the future?

type ForeachArguments struct {
	Collection string `alloy:"collection,attr`
	//TODO: Is the "var" argument really needed?
	// We could just have a variable with a fixed name referencing the current thing we are iterating over.
	Var string `alloy:"var,attr,optional`
}

func NewForeachConfigNode(block *ast.BlockStmt, globals ComponentGlobals) *ForeachConfigNode {
	nodeID := BlockComponentID(block).String()

	return &ForeachConfigNode{
		nodeID: nodeID,
		label:  block.Label,
		block:  block,
	}
}

func (fn *ForeachConfigNode) Label() string { return fn.label }

func (fn *ForeachConfigNode) NodeID() string { return fn.nodeID }

func (fn *ForeachConfigNode) Block() *ast.BlockStmt { return fn.block }

func (fn *ForeachConfigNode) Evaluate(scope *vm.Scope) error {
	return nil
}

func (fn *ForeachConfigNode) UpdateBlock(b *ast.BlockStmt) {
}
