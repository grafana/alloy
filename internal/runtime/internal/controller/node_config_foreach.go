package controller

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

type ForeachConfigNode struct {
	nodeID           string
	label            string
	moduleController ModuleController

	customComponents map[string]CustomComponent

	forEachChildrenUpdateChan chan struct{} // used to trigger an update of the running children
	forEachChildrenRunning    bool

	mut   sync.RWMutex
	block *ast.BlockStmt
	eval  *vm.Evaluator
}

var _ BlockNode = (*ForeachConfigNode)(nil)
var _ RunnableNode = (*ForeachConfigNode)(nil)

// For now the Foreach doesn't have the ability to export arguments.
//TODO: We could implement this in the future?

type ForeachArguments struct {
	Collection []string `alloy:"collection,attr"`
}

func NewForeachConfigNode(block *ast.BlockStmt, globals ComponentGlobals) *ForeachConfigNode {
	nodeID := BlockComponentID(block).String()
	globalID := nodeID

	return &ForeachConfigNode{
		nodeID:                    nodeID,
		label:                     block.Label,
		block:                     block,
		eval:                      vm.New(block.Body),
		moduleController:          globals.NewModuleController(globalID),
		forEachChildrenUpdateChan: make(chan struct{}, 1),
		customComponents:          make(map[string]CustomComponent, 0),
	}
}

func (fn *ForeachConfigNode) Label() string { return fn.label }

func (fn *ForeachConfigNode) NodeID() string { return fn.nodeID }

func (fn *ForeachConfigNode) Block() *ast.BlockStmt { return fn.block }

func (fn *ForeachConfigNode) Evaluate(scope *vm.Scope) error {
	fn.mut.Lock()
	defer fn.mut.Unlock()

	//TODO: Get the "template" block
	//TODO: Prefix the custom components with something like "foreach.testForeach.1."
	//TODO: find a way to evaluate the block?
	collection, template, err := getArgs(fn.block.Body)
	if err != nil {
		return fmt.Errorf("parsing foreach block: %w", err)
	}

	//TODO: Take into account the actual items in the collection.
	// The custom components should be able to use the values from the collection.
	loopCount := len(collection)

	// Loop through the items to create the custom components.
	// On re-evaluation new components are added and existing ones are updated.
	newCustomComponentIds := make(map[string]bool, loopCount)
	tmp := []string{"aaa", "bbb", "ccc", "ddd"}
	for i := 0; i < loopCount; i++ {
		customComponentID := tmp[i]
		cc, err := fn.getOrCreateCustomComponent(customComponentID)
		if err != nil {
			return err
		}

		args := map[string]any{}
		// TODO: use the registry from the loader to access the modules
		customComponentRegistry := NewCustomComponentRegistry(nil, scope)
		if err := cc.LoadBody(template, args, customComponentRegistry); err != nil {
			return fmt.Errorf("updating custom component in foreach: %w", err)
		}
		newCustomComponentIds[customComponentID] = true
	}

	// Delete the custom components that are no longer in the foreach.
	// The runner pkg will stop them properly.
	for id := range fn.customComponents {
		if _, exist := newCustomComponentIds[id]; !exist {
			delete(fn.customComponents, id)
		}
	}

	// trigger to stop previous children from running and to start running the new ones.
	if fn.forEachChildrenRunning {
		select {
		case fn.forEachChildrenUpdateChan <- struct{}{}: // queued trigger
		default: // trigger already queued; no-op
		}
	}
	return nil
}

func (fn *ForeachConfigNode) getOrCreateCustomComponent(customComponentID string) (CustomComponent, error) {
	cc, exists := fn.customComponents[customComponentID]
	if exists {
		return cc, nil
	}

	newCC, err := fn.moduleController.NewCustomComponent(customComponentID, func(exports map[string]any) {})
	if err != nil {
		return nil, fmt.Errorf("creating custom component: %w", err)
	}
	fn.customComponents[customComponentID] = newCC
	return newCC, nil
}

func (fn *ForeachConfigNode) UpdateBlock(b *ast.BlockStmt) {
	fn.mut.Lock()
	defer fn.mut.Unlock()
	fn.block = b
}

func (fn *ForeachConfigNode) Run(ctx context.Context) error {
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := runner.New(func(forEachChild *forEachChild) runner.Worker {
		return &forEachChildRunner{
			child: forEachChild,
		}
	})
	defer runner.Stop()

	updateTasks := func() error {
		fn.mut.Lock()
		defer fn.mut.Unlock()
		fn.forEachChildrenRunning = true
		var tasks []*forEachChild
		for customComponentID, customComponent := range fn.customComponents {
			tasks = append(tasks, &forEachChild{
				id: customComponentID,
				cc: customComponent,
			})
		}

		return runner.ApplyTasks(newCtx, tasks)
	}

	err := updateTasks()
	if err != nil {
		// TODO: log error
	}

	return fn.run(ctx, updateTasks)
}

func (fn *ForeachConfigNode) run(ctx context.Context, updateTasks func() error) error {
	for {
		select {
		case <-fn.forEachChildrenUpdateChan:
			err := updateTasks()
			if err != nil {
				// TODO: log error
			}
		case <-ctx.Done():
			return nil
		}
	}
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
			if stmt.GetBlockName() != "template" {
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

type forEachChildRunner struct {
	child *forEachChild
}

type forEachChild struct {
	cc CustomComponent
	id string
}

func (fr *forEachChildRunner) Run(ctx context.Context) {
	err := fr.child.cc.Run(ctx)
	if err != nil {
		// TODO: log and update health
	}
}

func (fi *forEachChild) Hash() uint64 {
	fnvHash := fnv.New64a()
	fnvHash.Write([]byte(fi.id))
	return fnvHash.Sum64()
}

func (fi *forEachChild) Equals(other runner.Task) bool {
	return fi.id == other.(*forEachChild).id
}
