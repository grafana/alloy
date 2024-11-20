package controller

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

type ForeachConfigNode struct {
	nodeID           string
	label            string
	block            *ast.BlockStmt // Current Alloy blocks to derive config from
	moduleController ModuleController
	customComponents []CustomComponent
}

var _ BlockNode = (*ForeachConfigNode)(nil)
var _ RunnableNode = (*ForeachConfigNode)(nil)

// For now the Foreach doesn't have the ability to export arguments.
//TODO: We could implement this in the future?

type ForeachArguments struct {
	Collection string `alloy:"collection,attr`
}

func NewForeachConfigNode(block *ast.BlockStmt, globals ComponentGlobals) *ForeachConfigNode {
	nodeID := BlockComponentID(block).String()
	globalID := nodeID

	return &ForeachConfigNode{
		nodeID:           nodeID,
		label:            block.Label,
		block:            block,
		moduleController: globals.NewModuleController(globalID),
	}
}

func (fn *ForeachConfigNode) Label() string { return fn.label }

func (fn *ForeachConfigNode) NodeID() string { return fn.nodeID }

func (fn *ForeachConfigNode) Block() *ast.BlockStmt { return fn.block }

func (fn *ForeachConfigNode) Evaluate(scope *vm.Scope) error {
	cc, err := fn.moduleController.NewCustomComponent("", func(exports map[string]any) {})
	if err != nil {
		return fmt.Errorf("creating custom component: %w", err)
	}

	//TODO: Get the "template" block
	//TODO: Prefix the custom components with something like "foreach.testForeach.1."
	collection, template, err := getArgs(fn.block.Body)

	//TODO: Take into account the actual items in the collection.
	// The custom components should be able to use the values from the collection.
	loopCount := len(collection)

	for i := 0; i < loopCount; i++ {
		args := map[string]any{}
		customComponentRegistry := NewCustomComponentRegistry(nil, scope)
		if err := cc.LoadBody(template, args, customComponentRegistry); err != nil {
			return fmt.Errorf("updating custom component: %w", err)
		}

		fn.customComponents = append(fn.customComponents, cc)
	}
	return nil
}

func getArgs(body ast.Body) ([]ast.Expr, ast.Body, error) {
	var collection []ast.Expr
	var template ast.Body

	if len(body) != 2 {
		return nil, nil, fmt.Errorf("foreach block must have two children")
	}

	for _, stmt := range body {
		switch stmt := stmt.(type) {
		case *ast.BlockStmt:
			if stmt.Label != "template" {
				return nil, nil, fmt.Errorf("unknown block")
			}
			template = stmt.Body
		case *ast.AttributeStmt:
			if stmt.Name.Name != "collection" {
				return nil, nil, fmt.Errorf("unknown attribute")
			}
			attrExpr, ok := stmt.Value.(*ast.ArrayExpr)
			if !ok {
				return nil, nil, fmt.Errorf("collection must be an array")
			}
			collection = attrExpr.Elements

		default:
			return nil, nil, fmt.Errorf("unknown argument")
		}
	}

	return collection, template, nil
}

func (fn *ForeachConfigNode) UpdateBlock(b *ast.BlockStmt) {
}

func (fn *ForeachConfigNode) Run(ctx context.Context) error {
	wg := &sync.WaitGroup{}
	for _, cc := range fn.customComponents {
		wg.Add(1)
		go func(cc CustomComponent) {
			defer wg.Done()
			//TODO: Get the error
			cc.Run(ctx)
		}(cc)
	}
	//TODO: Return all the errors from Run functions which failed
	return nil
}
