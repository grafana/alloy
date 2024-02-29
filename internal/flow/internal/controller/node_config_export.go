package controller

import (
	"fmt"
	"sync"

	"github.com/grafana/river/ast"
	"github.com/grafana/river/vm"
)

type ExportConfigNode struct {
	label         string
	nodeID        string
	componentName string

	mut   sync.RWMutex
	block *ast.BlockStmt // Current River blocks to derive config from
	eval  *vm.Evaluator
	value any
}

var _ BlockNode = (*ExportConfigNode)(nil)

// NewExportConfigNode creates a new ExportConfigNode from an initial ast.BlockStmt.
// The underlying config isn't applied until Evaluate is called.
func NewExportConfigNode(block *ast.BlockStmt, globals ComponentGlobals) *ExportConfigNode {
	return &ExportConfigNode{
		label:         block.Label,
		nodeID:        BlockComponentID(block).String(),
		componentName: block.GetBlockName(),

		block: block,
		eval:  vm.New(block.Body),
	}
}

type exportBlock struct {
	Value any `river:"value,attr"`
}

// Evaluate implements BlockNode and updates the arguments for the managed config block
// by re-evaluating its River block with the provided scope. The managed config block
// will be built the first time Evaluate is called.
//
// Evaluate will return an error if the River block cannot be evaluated or if
// decoding to arguments fails.
func (cn *ExportConfigNode) Evaluate(scope *vm.Scope) error {
	cn.mut.Lock()
	defer cn.mut.Unlock()

	var export exportBlock
	if err := cn.eval.Evaluate(scope, &export); err != nil {
		return fmt.Errorf("decoding River: %w", err)
	}
	cn.value = export.Value
	return nil
}

func (cn *ExportConfigNode) Label() string { return cn.label }

// Value returns the value of the export.
func (cn *ExportConfigNode) Value() any {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.value
}

// Block implements BlockNode and returns the current block of the managed config node.
func (cn *ExportConfigNode) Block() *ast.BlockStmt {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.block
}

// NodeID implements dag.Node and returns the unique ID for the config node.
func (cn *ExportConfigNode) NodeID() string { return cn.nodeID }
