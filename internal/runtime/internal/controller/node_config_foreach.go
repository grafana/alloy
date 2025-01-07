package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"path"
	"sync"

	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

type ForeachConfigNode struct {
	nodeID           string
	label            string
	moduleController ModuleController

	customComponents          map[string]CustomComponent
	customComponentHashCounts map[string]int

	forEachChildrenUpdateChan chan struct{} // used to trigger an update of the running children
	forEachChildrenRunning    bool

	mut   sync.RWMutex
	block *ast.BlockStmt
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
	if globals.ControllerID != "" {
		globalID = path.Join(globals.ControllerID, nodeID)
	}

	return &ForeachConfigNode{
		nodeID:                    nodeID,
		label:                     block.Label,
		block:                     block,
		moduleController:          globals.NewModuleController(globalID),
		forEachChildrenUpdateChan: make(chan struct{}, 1),
		customComponents:          make(map[string]CustomComponent, 0),
		customComponentHashCounts: make(map[string]int, 0),
	}
}

func (fn *ForeachConfigNode) Label() string { return fn.label }

func (fn *ForeachConfigNode) NodeID() string { return fn.nodeID }

func (fn *ForeachConfigNode) Block() *ast.BlockStmt { return fn.block }

type Arguments struct {
	Collection []any  `alloy:"collection,attr"`
	Var        string `alloy:"var,attr"`
}

func (fn *ForeachConfigNode) Evaluate(scope *vm.Scope) error {
	fn.mut.Lock()
	defer fn.mut.Unlock()

	var argsBody ast.Body
	var template *ast.BlockStmt
	for _, stmt := range fn.block.Body {
		if blockStmt, ok := stmt.(*ast.BlockStmt); ok && blockStmt.GetBlockName() == "template" {
			template = blockStmt
			continue // we don't add the template to the argsBody
		}
		argsBody = append(argsBody, stmt)
	}

	if template == nil {
		return fmt.Errorf("the block template is missing in the foreach block")
	}

	eval := vm.New(argsBody)

	var args Arguments
	if err := eval.Evaluate(scope, &args); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	// Loop through the items to create the custom components.
	// On re-evaluation new components are added and existing ones are updated.
	newCustomComponentIds := make(map[string]bool, len(args.Collection))
	fn.customComponentHashCounts = make(map[string]int)
	for i := 0; i < len(args.Collection); i++ {

		// We must create an ID from the collection entries to avoid recreating all components on every updates.
		// We track the hash counts because the collection might contain duplicates ([1, 1, 1] would result in the same ids
		// so we handle it by adding the count at the end -> [11, 12, 13]
		customComponentID := fmt.Sprintf("foreach_%s", hashObject(args.Collection[i]))
		count := fn.customComponentHashCounts[customComponentID] // count = 0 if the key is not found
		fn.customComponentHashCounts[customComponentID] = count + 1
		customComponentID += fmt.Sprintf("_%d", count+1)

		cc, err := fn.getOrCreateCustomComponent(customComponentID)
		if err != nil {
			return err
		}

		vars := deepCopyMap(scope.Variables)
		vars[args.Var] = args.Collection[i]

		// TODO: use the registry from the loader to access the modules
		customComponentRegistry := NewCustomComponentRegistry(nil, vm.NewScope(vars))
		if err := cc.LoadBody(template.Body, map[string]any{}, customComponentRegistry); err != nil {
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

func computeHash(s string) string {
	hasher := sha256.New()
	hasher.Write([]byte(s))
	fullHash := hasher.Sum(nil)
	return hex.EncodeToString(fullHash[:12]) // taking only the 12 first char of the hash should be enough
}

func hashObject(obj any) string {
	switch v := obj.(type) {
	case int, string, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		return computeHash(fmt.Sprintf("%#v", v))
	}
}
