package controller

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

// getCustomComponentConfig is used by the custom component to retrieve its template and the customComponentRegistry associated with it.
type getCustomComponentConfig func(namespace string, componentName string) (ast.Body, *CustomComponentRegistry, error)

// CustomComponentNode is a controller node which manages a custom component.
//
// CustomComponentNode manages the underlying custom component and caches its current
// arguments and exports.
type CustomComponentNode struct {
	id                ComponentID
	globalID          string
	label             string
	componentName     string
	nodeID            string // Cached from id.String() to avoid allocating new strings every time NodeID is called.
	moduleController  ModuleController
	OnBlockNodeUpdate func(cn BlockNode) // Informs controller that we need to reevaluate
	logger            log.Logger

	importNamespace     string
	customComponentName string

	getConfig getCustomComponentConfig // Retrieve the custom component config.

	mut     sync.RWMutex
	block   *ast.BlockStmt // Current Alloy block to derive args from
	eval    *vm.Evaluator
	managed CustomComponent     // Inner managed custom component
	args    component.Arguments // Evaluated arguments for the managed component

	// NOTE(rfratto): health and exports have their own mutex because they may be
	// set asynchronously while mut is still being held (i.e., when calling Evaluate
	// and the managed custom component immediately creates new exports)

	healthMut  sync.RWMutex
	evalHealth component.Health // Health of the last evaluate
	runHealth  component.Health // Health of running the component

	exportsMut sync.RWMutex
	exports    component.Exports // Evaluated exports for the managed custom component
}

var _ ComponentNode = (*CustomComponentNode)(nil)

// ExtractImportAndDeclare extracts an importNamespace and a customComponentName from a componentName.
// TODO: this function assumes that CustomComponentNames and ImportNamespaces don't contain "."
// In other words, there are two possible scenarios:
// - [customComponentName] LABEL ->  instance of a local declare
// - [importNamespace].[customComponentName] LABEL -> instance of an imported declare
// To address this limitation in the future, we will need to find a different approach to
// identify the importNamespaces and CustomComponentNames.
func ExtractImportAndDeclare(componentName string) (importNamespace, customComponentName string) {
	parts := strings.SplitN(componentName, ".", 2)
	switch len(parts) {
	case 1: // [customComponentName]
		customComponentName = parts[0]
	case 2: // [importNamespace].[customComponentName]
		importNamespace = parts[0]
		customComponentName = parts[1]
	}

	return importNamespace, customComponentName
}

// NewCustomComponentNode creates a new CustomComponentNode from an initial ast.BlockStmt.
// The underlying managed custom component isn't created until Evaluate is called.
func NewCustomComponentNode(globals ComponentGlobals, b *ast.BlockStmt, getConfig getCustomComponentConfig) *CustomComponentNode {
	var (
		id     = BlockComponentID(b)
		nodeID = id.String()
	)

	initHealth := component.Health{
		Health:     component.HealthTypeUnknown,
		Message:    "node custom component created",
		UpdateTime: time.Now(),
	}

	// We need to generate a globally unique component ID to give to the
	// component and for use with telemetry data which doesn't support
	// reconstructing the global ID. For everything else (HTTP, data), we can
	// just use the controller-local ID as those values are guaranteed to be
	// globally unique.
	globalID := nodeID
	if globals.ControllerID != "" {
		globalID = path.Join(globals.ControllerID, nodeID)
	}

	componentName := b.GetBlockName()
	importNamespace, customComponentName := ExtractImportAndDeclare(componentName)
	parent, node := splitPath(globalID)

	cn := &CustomComponentNode{
		id:                  id,
		globalID:            globalID,
		label:               b.Label,
		nodeID:              nodeID,
		componentName:       componentName,
		importNamespace:     importNamespace,
		customComponentName: customComponentName,
		moduleController:    globals.NewModuleController(globalID),
		OnBlockNodeUpdate:   globals.OnBlockNodeUpdate,
		logger:              log.With(globals.Logger, "component_path", parent, "component_id", node),
		getConfig:           getConfig,

		block: b,
		eval:  vm.New(b.Body),

		evalHealth: initHealth,
		runHealth:  initHealth,
	}

	return cn
}

// ID returns the component ID of the managed component from its Alloy block.
func (cn *CustomComponentNode) ID() ComponentID { return cn.id }

// Label returns the label for the block or "" if none was specified.
func (cn *CustomComponentNode) Label() string { return cn.label }

// NodeID implements dag.Node and returns the unique ID for this node. The
// NodeID is the string representation of the component's ID from its Alloy
// block.
func (cn *CustomComponentNode) NodeID() string { return cn.nodeID }

// UpdateBlock updates the Alloy block used to construct arguments for the
// managed component. The new block isn't used until the next time Evaluate is
// invoked.
//
// UpdateBlock will panic if the block does not match the component ID of the
// CustomComponentNode.
func (cn *CustomComponentNode) UpdateBlock(b *ast.BlockStmt) {
	if !BlockComponentID(b).Equals(cn.id) {
		panic("UpdateBlock called with an Alloy block with a different component ID")
	}

	cn.mut.Lock()
	defer cn.mut.Unlock()
	cn.block = b
	cn.eval = vm.New(b.Body)
}

// Evaluate implements BlockNode and updates the arguments by re-evaluating its Alloy block with the provided scope and the custom component by
// retrieving the component definition from the corresponding import or declare node.
// The managed custom component will be built the first time Evaluate is called.
//
// Evaluate will return an error if the Alloy block cannot be evaluated, if
// decoding to arguments fails or if the custom component definition cannot be retrieved.
func (cn *CustomComponentNode) Evaluate(evalScope *vm.Scope) error {
	err := cn.evaluate(evalScope)

	switch err {
	case nil:
		cn.setEvalHealth(component.HealthTypeHealthy, "component evaluated")
	default:
		msg := fmt.Sprintf("component evaluation failed: %s", err)
		cn.setEvalHealth(component.HealthTypeUnhealthy, msg)
	}
	return err
}

func (cn *CustomComponentNode) evaluate(evalScope *vm.Scope) error {
	cn.mut.Lock()
	defer cn.mut.Unlock()

	var args map[string]any
	if err := cn.eval.Evaluate(evalScope, &args); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	cn.args = args

	if cn.managed == nil {
		// We haven't built the managed custom component successfully yet.
		mod, err := cn.moduleController.NewCustomComponent("", func(exports map[string]any) { cn.setExports(exports) })
		if err != nil {
			return fmt.Errorf("creating custom component controller: %w", err)
		}
		cn.managed = mod
	}

	template, customComponentRegistry, err := cn.getConfig(cn.importNamespace, cn.customComponentName)
	if err != nil {
		return fmt.Errorf("loading custom component controller: %w", err)
	}

	// Reload the custom component with new config
	if err := cn.managed.LoadBody(template, args, customComponentRegistry); err != nil {
		return fmt.Errorf("updating custom component: %w", err)
	}
	return nil
}

func (cn *CustomComponentNode) Run(ctx context.Context) error {
	cn.mut.RLock()
	managed := cn.managed
	cn.mut.RUnlock()

	if managed == nil {
		return ErrUnevaluated
	}

	cn.setRunHealth(component.HealthTypeHealthy, "started custom component")
	err := managed.Run(ctx)

	// Note: logging of this error is handled by the scheduler.
	if err != nil {
		cn.setRunHealth(component.HealthTypeExited, fmt.Sprintf("custom component shut down with error: %s", err))
	} else {
		cn.setRunHealth(component.HealthTypeExited, "custom component shut down cleanly")
	}
	return err
}

// Arguments returns the current arguments of the managed custom component.
func (cn *CustomComponentNode) Arguments() component.Arguments {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.args
}

// Block implements BlockNode and returns the current block of the managed custom component.
func (cn *CustomComponentNode) Block() *ast.BlockStmt {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.block
}

// Exports returns the current set of exports from the managed custom component.
// Exports returns nil if the managed custom component does not have exports.
func (cn *CustomComponentNode) Exports() component.Exports {
	cn.exportsMut.RLock()
	defer cn.exportsMut.RUnlock()
	return cn.exports
}

// setExports is called whenever the managed custom component updates. e must be the
// same type as the registered exports type of the managed custom component.
func (cn *CustomComponentNode) setExports(e component.Exports) {
	// Some components may aggressively reexport values even though no exposed
	// state has changed. This may be done for components which always supply
	// exports whenever their arguments are evaluated without tracking internal
	// state to see if anything actually changed.
	//
	// To avoid needlessly reevaluating components we'll ignore unchanged
	// exports.
	var changed bool

	cn.exportsMut.Lock()
	if !reflect.DeepEqual(cn.exports, e) {
		changed = true
		cn.exports = e
	}
	cn.exportsMut.Unlock()

	if changed {
		// Inform the controller that we have new exports.
		cn.OnBlockNodeUpdate(cn)
	}
}

// CurrentHealth returns the current health of the CustomComponentNode.
//
// The health of a CustomComponentNode is determined by combining:
//
//  1. Health from the call to Run().
//  2. Health from the last call to Evaluate().
func (cn *CustomComponentNode) CurrentHealth() component.Health {
	cn.healthMut.RLock()
	defer cn.healthMut.RUnlock()
	return component.LeastHealthy(cn.runHealth, cn.evalHealth)
}

// setEvalHealth sets the internal health from a call to Evaluate. See Health
// for information on how overall health is calculated.
func (cn *CustomComponentNode) setEvalHealth(t component.HealthType, msg string) {
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
func (cn *CustomComponentNode) setRunHealth(t component.HealthType, msg string) {
	cn.healthMut.Lock()
	defer cn.healthMut.Unlock()

	cn.runHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

// ComponentName returns the name of the component.
func (cn *CustomComponentNode) ComponentName() string {
	return cn.componentName
}

// TODO: currently used by the component provider to access the components running within
// the custom components. Change it when getting rid of old modules.
func (cn *CustomComponentNode) ModuleIDs() []string {
	return cn.moduleController.ModuleIDs()
}
