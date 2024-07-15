package controller

import (
	"context"
	"fmt"
	"hash/fnv"
	"maps"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/internal/importsource"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

// ImportConfigNode imports declare and import blocks via a managed import source.
// The imported declare are stored in importedDeclares.
// For every imported import block, the ImportConfigNode will create ImportConfigNode children.
// The children are evaluated and ran by the parent.
// When an ImportConfigNode receives new content from its source, it updates its importedDeclares and recreates its children.
// Then an update call is propagated to the root ImportConfigNode to inform the controller for reevaluation.
type ImportConfigNode struct {
	nodeID        string
	globalID      string
	label         string
	componentName string
	globals       ComponentGlobals          // Need a copy of the globals to create other import nodes
	block         *ast.BlockStmt            // Current Alloy blocks to derive config from
	source        importsource.ImportSource // source retrieves the module content
	registry      *prometheus.Registry

	OnBlockNodeUpdate func(cn BlockNode) // notifies the controller or the parent for reevaluation
	logger            log.Logger

	importChildrenUpdateChan chan struct{} // used to trigger an update of the running children

	mut                       sync.RWMutex
	importedContent           map[string]string
	importConfigNodesChildren map[string]*ImportConfigNode
	importChildrenRunning     bool
	importedDeclares          map[string]ast.Body

	healthMut     sync.RWMutex
	evalHealth    component.Health // Health of the last source evaluation
	runHealth     component.Health // Health of running
	contentHealth component.Health // Health of the last content update

	inContentUpdate atomic.Bool
}

var _ RunnableNode = (*ImportConfigNode)(nil)

// NewImportConfigNode creates a new ImportConfigNode from an initial ast.BlockStmt.
// The underlying config isn't applied until Evaluate is called.
func NewImportConfigNode(block *ast.BlockStmt, globals ComponentGlobals, sourceType importsource.SourceType) *ImportConfigNode {
	nodeID := BlockComponentID(block).String()

	globalID := nodeID
	if globals.ControllerID != "" {
		globalID = path.Join(globals.ControllerID, nodeID)
	}

	cn := &ImportConfigNode{
		nodeID:                   nodeID,
		globalID:                 globalID,
		label:                    block.Label,
		componentName:            block.GetBlockName(),
		globals:                  globals,
		block:                    block,
		OnBlockNodeUpdate:        globals.OnBlockNodeUpdate,
		importChildrenUpdateChan: make(chan struct{}, 1),
	}
	managedOpts := getImportManagedOptions(globals, cn)
	cn.logger = managedOpts.Logger
	cn.source = importsource.NewImportSource(sourceType, managedOpts, vm.New(block.Body), cn.onContentUpdate)
	return cn
}

func getImportManagedOptions(globals ComponentGlobals, cn *ImportConfigNode) component.Options {
	cn.registry = prometheus.NewRegistry()
	parent, id := splitPath(cn.globalID)
	return component.Options{
		ID:     cn.globalID,
		Logger: log.With(globals.Logger, "config_path", parent, "config_id", id),
		Registerer: prometheus.WrapRegistererWith(prometheus.Labels{
			"config_path": parent,
			"config_id":   id,
		}, cn.registry),
		Tracer:   tracing.WrapTracer(globals.TraceProvider, cn.globalID),
		DataPath: filepath.Join(globals.DataPath, cn.globalID),
		GetServiceData: func(name string) (interface{}, error) {
			return globals.GetServiceData(name)
		},
	}
}

// setEvalHealth sets the internal health from a call to Evaluate. See Health
// for information on how overall health is calculated.
func (cn *ImportConfigNode) setEvalHealth(t component.HealthType, msg string) {
	cn.healthMut.Lock()
	defer cn.healthMut.Unlock()

	cn.evalHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

// setRunHealth sets the internal health from a call to Run. See Health for
// information on how overall health is calculated.
func (cn *ImportConfigNode) setRunHealth(t component.HealthType, msg string) {
	cn.healthMut.Lock()
	defer cn.healthMut.Unlock()

	cn.runHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

// setContentHealth sets the internal health from a call to OnContentUpdate. See Health
// for information on how overall health is calculated.
func (cn *ImportConfigNode) setContentHealth(t component.HealthType, msg string) {
	cn.healthMut.Lock()
	defer cn.healthMut.Unlock()

	cn.contentHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

// CurrentHealth returns the current health of the ImportConfigNode.
//
// The health of a ImportConfigNode is determined by combining:
//
//  1. Health from the call to Run().
//  2. Health from the last call to Evaluate().
//  3. Health from the last call to OnContentChange().
//  4. Health reported from the source.
//  5. Health reported from the nested imports.
func (cn *ImportConfigNode) CurrentHealth() component.Health {
	cn.healthMut.RLock()
	defer cn.healthMut.RUnlock()
	cn.mut.RLock()
	defer cn.mut.RUnlock()

	health := component.LeastHealthy(
		cn.runHealth,
		cn.evalHealth,
		cn.contentHealth,
		cn.source.CurrentHealth(),
	)

	for _, child := range cn.importConfigNodesChildren {
		health = component.LeastHealthy(health, child.CurrentHealth())
	}

	return health
}

// Evaluate implements BlockNode and evaluates the import source.
func (cn *ImportConfigNode) Evaluate(scope *vm.Scope) error {
	err := cn.source.Evaluate(scope)
	switch err {
	case nil:
		cn.setEvalHealth(component.HealthTypeHealthy, "source evaluated")
	default:
		msg := fmt.Sprintf("source evaluation failed: %s", err)
		cn.setEvalHealth(component.HealthTypeUnhealthy, msg)
	}
	return err
}

// onContentUpdate is triggered every time the managed import source has new content.
func (cn *ImportConfigNode) onContentUpdate(importedContent map[string]string) {
	cn.mut.Lock()
	defer cn.mut.Unlock()

	cn.inContentUpdate.Store(true)
	defer cn.inContentUpdate.Store(false)

	// If the source sent the same content, there is no need to reload.
	if maps.Equal(cn.importedContent, importedContent) {
		return
	}

	cn.importedContent = make(map[string]string)
	for k, v := range importedContent {
		cn.importedContent[k] = v
	}
	cn.importedDeclares = make(map[string]ast.Body)
	cn.importConfigNodesChildren = make(map[string]*ImportConfigNode)

	for f, ic := range importedContent {
		parsedImportedContent, err := parser.ParseFile(cn.label, []byte(ic))
		if err != nil {
			level.Error(cn.logger).Log("msg", "failed to parse file on update", "file", f, "err", err)
			cn.setContentHealth(component.HealthTypeUnhealthy, fmt.Sprintf("imported content from %q cannot be parsed: %s", f, err))
			return
		}

		// populate importedDeclares and importConfigNodesChildren
		err = cn.processImportedContent(parsedImportedContent)
		if err != nil {
			level.Error(cn.logger).Log("msg", "failed to process imported content", "file", f, "err", err)
			cn.setContentHealth(component.HealthTypeUnhealthy, fmt.Sprintf("imported content from %q is invalid: %s", f, err))
			return
		}
	}

	// evaluate the importConfigNodesChildren that have been created
	err := cn.evaluateChildren()
	if err != nil {
		level.Error(cn.logger).Log("msg", "failed to evaluate nested import", "err", err)
		cn.setContentHealth(component.HealthTypeUnhealthy, fmt.Sprintf("nested import block failed to evaluate: %s", err))
		return
	}

	// trigger to stop previous children from running and to start running the new ones.
	if cn.importChildrenRunning {
		select {
		case cn.importChildrenUpdateChan <- struct{}{}: // queued trigger
		default: // trigger already queued; no-op
		}
	}

	cn.setContentHealth(component.HealthTypeHealthy, "content updated")
	cn.OnBlockNodeUpdate(cn)
}

// processImportedContent processes declare and import blocks of the provided ast content.
func (cn *ImportConfigNode) processImportedContent(content *ast.File) error {
	for _, stmt := range content.Body {
		blockStmt, ok := stmt.(*ast.BlockStmt)
		if !ok {
			return fmt.Errorf("only declare and import blocks are allowed in a module")
		}

		componentName := strings.Join(blockStmt.Name, ".")
		switch componentName {
		case declareType:
			cn.processDeclareBlock(blockStmt)
		case importsource.BlockImportFile, importsource.BlockImportString, importsource.BlockImportHTTP, importsource.BlockImportGit:
			err := cn.processImportBlock(blockStmt, componentName)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("only declare and import blocks are allowed in a module, got %s", componentName)
		}
	}
	return nil
}

// processDeclareBlock stores the declare definition in the importedDeclares.
func (cn *ImportConfigNode) processDeclareBlock(stmt *ast.BlockStmt) {
	if _, ok := cn.importedDeclares[stmt.Label]; ok {
		level.Error(cn.logger).Log("msg", "declare block redefined", "name", stmt.Label)
		return
	}
	cn.importedDeclares[stmt.Label] = stmt.Body
}

// processDeclareBlock creates an ImportConfigNode child from the provided import block.
func (cn *ImportConfigNode) processImportBlock(stmt *ast.BlockStmt, fullName string) error {
	sourceType := importsource.GetSourceType(fullName)
	if _, ok := cn.importConfigNodesChildren[stmt.Label]; ok {
		return fmt.Errorf("import block redefined %s", stmt.Label)
	}
	childGlobals := cn.globals
	// Children have a special OnBlockNodeUpdate function which notifies the parent when its content changes.
	childGlobals.OnBlockNodeUpdate = cn.onChildrenContentUpdate
	// Children data paths are nested inside their parents to avoid collisions.
	childGlobals.DataPath = filepath.Join(childGlobals.DataPath, cn.globalID)
	cn.importConfigNodesChildren[stmt.Label] = NewImportConfigNode(stmt, childGlobals, sourceType)
	return nil
}

// evaluateChildren evaluates the import nodes managed by this import node.
func (cn *ImportConfigNode) evaluateChildren() error {
	for _, child := range cn.importConfigNodesChildren {
		err := child.Evaluate(&vm.Scope{
			Parent:    nil,
			Variables: make(map[string]interface{}),
		})
		if err != nil {
			return fmt.Errorf("imported node %s failed to evaluate, %v", child.label, err)
		}
	}
	return nil
}

// onChildrenContentUpdate notifies the parent that the content has been updated.
func (cn *ImportConfigNode) onChildrenContentUpdate(child BlockNode) {
	// If the node is already updating its content, it will call OnBlockNodeUpdate
	// so the notification can be ignored.
	if !cn.inContentUpdate.Load() {
		cn.OnBlockNodeUpdate(cn)
	}
}

// Run runs the managed source and the import children until ctx is
// canceled. Evaluate must have been called at least once without returning an
// error before calling Run.
//
// Run will immediately return ErrUnevaluated if Evaluate has never been called
// successfully. Otherwise, Run will return nil.
func (cn *ImportConfigNode) Run(ctx context.Context) error {
	if cn.source == nil {
		return ErrUnevaluated
	}

	newCtx, cancel := context.WithCancel(ctx)
	defer cancel() // This will stop the children and the managed source.

	errChan := make(chan error, 1)

	runner := runner.New(func(node *ImportConfigNode) runner.Worker {
		return &childRunner{
			node: node,
		}
	})
	defer runner.Stop()

	updateTasks := func() error {
		cn.mut.Lock()
		defer cn.mut.Unlock()
		cn.importChildrenRunning = true
		var tasks []*ImportConfigNode
		for _, value := range cn.importConfigNodesChildren {
			tasks = append(tasks, value)
		}

		return runner.ApplyTasks(newCtx, tasks)
	}

	cn.setRunHealth(component.HealthTypeHealthy, "started import")

	err := updateTasks()
	if err != nil {
		level.Error(cn.logger).Log("msg", "import failed to run nested imports", "err", err)
		cn.setRunHealth(component.HealthTypeUnhealthy, fmt.Sprintf("error encountered while running nested import blocks: %s", err))
		// the error is not fatal, the node can still run in unhealthy mode
	}

	go func() {
		errChan <- cn.source.Run(newCtx)
	}()

	err = cn.run(errChan, updateTasks)

	// Note: logging of this error is handled by the scheduler.
	if err != nil {
		cn.setRunHealth(component.HealthTypeExited, fmt.Sprintf("import shut down with error: %s", err))
	} else {
		cn.setRunHealth(component.HealthTypeExited, "import shut down cleanly")
	}
	return err
}

func (cn *ImportConfigNode) run(errChan chan error, updateTasks func() error) error {
	for {
		select {
		case <-cn.importChildrenUpdateChan:
			err := updateTasks()
			if err != nil {
				level.Error(cn.logger).Log("msg", "error encountered while updating nested import blocks", "err", err)
				cn.setRunHealth(component.HealthTypeUnhealthy, fmt.Sprintf("error encountered while updating nested import blocks: %s", err))
				// the error is not fatal, the node can still run in unhealthy mode
			} else {
				cn.setRunHealth(component.HealthTypeHealthy, "nested imports updated successfully")
			}
		case err := <-errChan:
			return err
		}
	}
}

// UpdateBlock updates the Alloy block used to construct arguments.
// The new block isn't used until the next time Evaluate is invoked.
//
// UpdateBlock will panic if the block does not match the component ID of the
// ImportConfigNode.
func (cn *ImportConfigNode) UpdateBlock(b *ast.BlockStmt) {
	if !BlockComponentID(b).Equals(strings.Split(cn.nodeID, ".")) {
		panic("UpdateBlock called with an Alloy block with a different ID")
	}

	cn.mut.Lock()
	defer cn.mut.Unlock()
	cn.block = b
	cn.source.SetEval(vm.New(b.Body))
}

func (cn *ImportConfigNode) Label() string { return cn.label }

// Block implements BlockNode and returns the current block of the managed config node.
func (cn *ImportConfigNode) Block() *ast.BlockStmt { return cn.block }

// NodeID implements dag.Node and returns the unique ID for the config node.
func (cn *ImportConfigNode) NodeID() string { return cn.nodeID }

// ImportedDeclares returns all declare blocks that it imported.
func (cn *ImportConfigNode) ImportedDeclares() map[string]ast.Body {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.importedDeclares
}

// ImportConfigNodesChildren returns the ImportConfigNodesChildren of this ImportConfigNode.
func (cn *ImportConfigNode) ImportConfigNodesChildren() map[string]*ImportConfigNode {
	cn.mut.Lock()
	defer cn.mut.Unlock()
	return cn.importConfigNodesChildren
}

type childRunner struct {
	node *ImportConfigNode
}

func (cr *childRunner) Run(ctx context.Context) {
	err := cr.node.Run(ctx)
	if err != nil {
		level.Error(cr.node.logger).Log("msg", "nested import stopped running", "err", err)
		cr.node.setRunHealth(component.HealthTypeUnhealthy, fmt.Sprintf("nested import stopped running: %s", err))
	}
}

func (cn *ImportConfigNode) Hash() uint64 {
	fnvHash := fnv.New64a()
	fnvHash.Write([]byte(cn.NodeID()))
	return fnvHash.Sum64()
}

// We don't want to reuse previous running tasks.
// On every updates, the previous workers should be stopped and new ones should spawn.
func (cn *ImportConfigNode) Equals(other runner.Task) bool {
	// pointers are exactly the same.
	// TODO: if possible we could find a way to safely reuse previous nodes
	return cn == other.(*ImportConfigNode)
}
