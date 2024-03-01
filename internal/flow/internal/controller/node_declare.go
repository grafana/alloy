package controller

import (
	"github.com/grafana/river/ast"
	"github.com/grafana/river/vm"
)

// DeclareNode represents a declare block in the DAG.
type DeclareNode struct {
	label         string
	nodeID        string
	componentName string
	block         *ast.BlockStmt
}

var _ BlockNode = (*DeclareNode)(nil)

const declareType = "declare"

// NewDeclareNode creates a new declare node with a content which will be loaded by custom components.
func NewDeclareNode(block *ast.BlockStmt) *DeclareNode {
	return &DeclareNode{
		label:         block.Label,
		nodeID:        BlockComponentID(block).String(),
		componentName: block.GetBlockName(),
		block:         block,
	}
}

// Evaluate does nothing for this node.
func (cn *DeclareNode) Evaluate(scope *vm.Scope) error {
	return nil
}

// Label returns the label of the block.
func (cn *DeclareNode) Label() string { return cn.label }

// Block implements BlockNode and returns the current block of the managed config node.
func (cn *DeclareNode) Block() *ast.BlockStmt {
	return cn.block
}

// NodeID implements dag.Node and returns the unique ID for the config node.
func (cn *DeclareNode) NodeID() string { return cn.nodeID }
