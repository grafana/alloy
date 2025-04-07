package controller

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

var _ BlockNode = (*LoggingConfigNode)(nil)

type LoggingConfigNode struct {
	nodeID        string
	componentName string
	l             log.Logger

	mut   sync.RWMutex
	block *ast.BlockStmt // Current Alloy blocks to derive config from
	eval  *vm.Evaluator
}

// NewLoggingConfigNode creates a new LoggingConfigNode from an initial ast.BlockStmt.
// The underlying config isn't applied until Evaluate is called.
func NewLoggingConfigNode(block *ast.BlockStmt, globals ComponentGlobals) *LoggingConfigNode {
	return &LoggingConfigNode{
		nodeID:        BlockComponentID(block).String(),
		componentName: block.GetBlockName(),
		l:             globals.Logger,

		block: block,
		eval:  vm.New(block.Body),
	}
}

// NewDefaultLoggingConfigNode creates a new LoggingConfigNode with nil block and eval.
// This will force evaluate to use the default logging options for this node.
func NewDefaultLoggingConfigNode(globals ComponentGlobals) *LoggingConfigNode {
	return &LoggingConfigNode{
		nodeID:        loggingBlockID,
		componentName: loggingBlockID,
		l:             globals.Logger,

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
func (cn *LoggingConfigNode) Evaluate(scope *vm.Scope) error {
	return cn.evaluate(scope, false)
}

func (cn *LoggingConfigNode) TypeCheck(scope *vm.Scope) error {
	return cn.evaluate(scope, true)
}

func (cn *LoggingConfigNode) evaluate(scope *vm.Scope, typeCheck bool) error {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	args := logging.DefaultOptions
	if cn.eval != nil {
		if err := cn.eval.Evaluate(scope, &args); err != nil {
			return fmt.Errorf("decoding configuration: %w", err)
		}
	}

	if typeCheck {
		return nil
	}

	if err := cn.l.(*logging.Logger).Update(args); err != nil {
		return fmt.Errorf("could not update logger: %w", err)
	}

	return nil
}

// Block implements BlockNode and returns the current block of the managed config node.
func (cn *LoggingConfigNode) Block() *ast.BlockStmt {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.block
}

// NodeID implements dag.Node and returns the unique ID for the config node.
func (cn *LoggingConfigNode) NodeID() string { return cn.nodeID }

// UpdateBlock updates the Alloy block used to construct arguments.
// The new block isn't used until the next time Evaluate is invoked.
//
// UpdateBlock will panic if the block does not match the component ID of the
// LoggingConfigNode.
func (cn *LoggingConfigNode) UpdateBlock(b *ast.BlockStmt) {
	if !BlockComponentID(b).Equals(strings.Split(cn.nodeID, ".")) {
		panic("UpdateBlock called with an Alloy block with a different ID")
	}

	cn.mut.Lock()
	defer cn.mut.Unlock()
	cn.block = b
	cn.eval = vm.New(b.Body)
}
