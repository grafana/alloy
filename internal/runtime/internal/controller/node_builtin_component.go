package controller

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

// ComponentID is a fully-qualified name of a component. Each element in
// ComponentID corresponds to a fragment of the period-delimited string;
// "remote.http.example" is ComponentID{"remote", "http", "example"}.
type ComponentID []string

// BlockComponentID returns the ComponentID specified by an Alloy block.
func BlockComponentID(b *ast.BlockStmt) ComponentID {
	id := make(ComponentID, 0, len(b.Name)+1) // add 1 for the optional label
	id = append(id, b.Name...)
	if b.Label != "" {
		id = append(id, b.Label)
	}
	return id
}

// String returns the string representation of a component ID.
func (id ComponentID) String() string {
	return strings.Join(id, ".")
}

// Equals returns true if id == other.
func (id ComponentID) Equals(other ComponentID) bool {
	if len(id) != len(other) {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] != other[i] {
			return false
		}
	}
	return true
}

// DialFunc is a function to establish a network connection.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// ComponentGlobals are used by BuiltinComponentNodes to build managed components. All
// BuiltinComponentNodes should use the same ComponentGlobals.
type ComponentGlobals struct {
	Logger               *logging.Logger                        // Logger shared between all managed components.
	TraceProvider        trace.TracerProvider                   // Tracer shared between all managed components.
	DataPath             string                                 // Shared directory where component data may be stored
	MinStability         featuregate.Stability                  // Minimum allowed stability level for features
	OnBlockNodeUpdate    func(cn BlockNode)                     // Informs controller that we need to reevaluate
	OnExportsChange      func(exports map[string]any)           // Invoked when the managed component updated its exports
	Registerer           prometheus.Registerer                  // Registerer for serving Alloy and component metrics
	ControllerID         string                                 // ID of controller.
	NewModuleController  func(id string) ModuleController       // Func to generate a module controller.
	GetServiceData       func(name string) (interface{}, error) // Get data for a service.
	EnableCommunityComps bool                                   // Enables the use of community components.
}

// BuiltinComponentNode is a controller node which manages a builtin component.
//
// BuiltinComponentNode manages the underlying component and caches its current
// arguments and exports. BuiltinComponentNode manages the arguments for the component
// from an Alloy block.
type BuiltinComponentNode struct {
	id                ComponentID
	globalID          string
	label             string
	componentName     string
	nodeID            string // Cached from id.String() to avoid allocating new strings every time NodeID is called.
	reg               component.Registration
	managedOpts       component.Options
	registry          *prometheus.Registry
	exportsType       reflect.Type
	moduleController  ModuleController
	OnBlockNodeUpdate func(cn BlockNode) // Informs controller that we need to reevaluate

	mut     sync.RWMutex
	block   *ast.BlockStmt // Current Alloy block to derive args from
	eval    *vm.Evaluator
	managed component.Component // Inner managed component
	args    component.Arguments // Evaluated arguments for the managed component

	// NOTE(rfratto): health and exports have their own mutex because they may be
	// set asynchronously while mut is still being held (i.e., when calling Evaluate
	// and the managed component immediately creates new exports)

	healthMut  sync.RWMutex
	evalHealth component.Health // Health of the last evaluate
	runHealth  component.Health // Health of running the component

	exportsMut sync.RWMutex
	exports    component.Exports // Evaluated exports for the managed component
}

var _ ComponentNode = (*BuiltinComponentNode)(nil)

// NewBuiltinComponentNode creates a new BuiltinComponentNode from an initial ast.BlockStmt.
// The underlying managed component isn't created until Evaluate is called.
func NewBuiltinComponentNode(globals ComponentGlobals, reg component.Registration, b *ast.BlockStmt) *BuiltinComponentNode {
	var (
		id     = BlockComponentID(b)
		nodeID = id.String()
	)

	initHealth := component.Health{
		Health:     component.HealthTypeUnknown,
		Message:    "component created",
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

	cn := &BuiltinComponentNode{
		id:                id,
		globalID:          globalID,
		label:             b.Label,
		nodeID:            nodeID,
		componentName:     strings.Join(b.Name, "."),
		reg:               reg,
		exportsType:       getExportsType(reg),
		moduleController:  globals.NewModuleController(globalID),
		OnBlockNodeUpdate: globals.OnBlockNodeUpdate,

		block: b,
		eval:  vm.New(b.Body),

		// Prepopulate arguments and exports with their zero values.
		args:    reg.Args,
		exports: reg.Exports,

		evalHealth: initHealth,
		runHealth:  initHealth,
	}
	cn.managedOpts = getManagedOptions(globals, cn)

	return cn
}

func getManagedOptions(globals ComponentGlobals, cn *BuiltinComponentNode) component.Options {
	cn.registry = prometheus.NewRegistry()
	parent, id := splitPath(cn.globalID)
	return component.Options{
		ID:     cn.globalID,
		Logger: log.With(globals.Logger, "component_path", parent, "component_id", id),
		Registerer: prometheus.WrapRegistererWith(prometheus.Labels{
			"component_path": parent,
			"component_id":   id,
		}, cn.registry),
		Tracer: tracing.WrapTracer(globals.TraceProvider, cn.globalID),

		DataPath: filepath.Join(globals.DataPath, cn.globalID),

		OnStateChange:    cn.setExports,
		ModuleController: cn.moduleController,

		GetServiceData: func(name string) (interface{}, error) {
			return globals.GetServiceData(name)
		},

		MinStability: globals.MinStability,
	}
}

func getExportsType(reg component.Registration) reflect.Type {
	if reg.Exports != nil {
		return reflect.TypeOf(reg.Exports)
	}
	return nil
}

// Registration returns the original registration of the component.
func (cn *BuiltinComponentNode) Registration() component.Registration { return cn.reg }

// Component returns the instance of the managed component. Component may be
// nil if the BuiltinComponentNode has not been successfully evaluated yet.
func (cn *BuiltinComponentNode) Component() component.Component {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.managed
}

// ID returns the component ID of the managed component from its Alloy block.
func (cn *BuiltinComponentNode) ID() ComponentID { return cn.id }

// Label returns the label for the block or "" if none was specified.
func (cn *BuiltinComponentNode) Label() string { return cn.label }

// ComponentName returns the component's type, i.e. `local.file.test` returns `local.file`.
func (cn *BuiltinComponentNode) ComponentName() string { return cn.componentName }

// NodeID implements dag.Node and returns the unique ID for this node. The
// NodeID is the string representation of the component's ID from its Alloy
// block.
func (cn *BuiltinComponentNode) NodeID() string { return cn.nodeID }

// UpdateBlock updates the Alloy block used to construct arguments for the
// managed component. The new block isn't used until the next time Evaluate is
// invoked.
//
// UpdateBlock will panic if the block does not match the component ID of the
// BuiltinComponentNode.
func (cn *BuiltinComponentNode) UpdateBlock(b *ast.BlockStmt) {
	if !BlockComponentID(b).Equals(cn.id) {
		panic("UpdateBlock called with an block with a different component ID")
	}

	cn.mut.Lock()
	defer cn.mut.Unlock()
	cn.block = b
	cn.eval = vm.New(b.Body)
}

// Evaluate implements BlockNode and updates the arguments for the managed component
// by re-evaluating its Alloy block with the provided scope. The managed component
// will be built the first time Evaluate is called.
//
// Evaluate will return an error if the Alloy block cannot be evaluated or if
// decoding to arguments fails.
func (cn *BuiltinComponentNode) Evaluate(scope *vm.Scope) error {
	err := cn.evaluate(scope)

	switch err {
	case nil:
		cn.setEvalHealth(component.HealthTypeHealthy, "component evaluated")
	default:
		msg := fmt.Sprintf("component evaluation failed: %s", err)
		cn.setEvalHealth(component.HealthTypeUnhealthy, msg)
	}
	return err
}

func (cn *BuiltinComponentNode) evaluate(scope *vm.Scope) error {
	cn.mut.Lock()
	defer cn.mut.Unlock()

	argsPointer := cn.reg.CloneArguments()
	if err := cn.eval.Evaluate(scope, argsPointer); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	// args is always a pointer to the args type, so we want to deference it since
	// components expect a non-pointer.
	argsCopyValue := reflect.ValueOf(argsPointer).Elem().Interface()

	if cn.managed == nil {
		// We haven't built the managed component successfully yet.
		managed, err := cn.reg.Build(cn.managedOpts, argsCopyValue)
		if err != nil {
			return fmt.Errorf("building component: %w", err)
		}
		cn.managed = managed
		cn.args = argsCopyValue

		return nil
	}

	if reflect.DeepEqual(cn.args, argsCopyValue) {
		// Ignore components which haven't changed. This reduces the cost of
		// calling evaluate for components where evaluation is expensive (e.g., if
		// re-evaluating requires re-starting some internal logic).
		return nil
	}

	// Update the existing managed component
	if err := cn.managed.Update(argsCopyValue); err != nil {
		return fmt.Errorf("updating component: %w", err)
	}

	cn.args = argsCopyValue
	return nil
}

// Run runs the managed component in the calling goroutine until ctx is
// canceled. Evaluate must have been called at least once without returning an
// error before calling Run.
//
// Run will immediately return ErrUnevaluated if Evaluate has never been called
// successfully. Otherwise, Run will return nil.
func (cn *BuiltinComponentNode) Run(ctx context.Context) error {
	cn.mut.RLock()
	managed := cn.managed
	cn.mut.RUnlock()

	if managed == nil {
		return ErrUnevaluated
	}

	cn.setRunHealth(component.HealthTypeHealthy, "started component")
	err := cn.managed.Run(ctx)

	// Note: logging of this error is handled by the scheduler.
	if err != nil {
		cn.setRunHealth(component.HealthTypeExited, fmt.Sprintf("component shut down with error: %s", err))
	} else {
		cn.setRunHealth(component.HealthTypeExited, "component shut down cleanly")
	}
	return err
}

// ErrUnevaluated is returned if BuiltinComponentNode.Run is called before a managed
// component is built.
var ErrUnevaluated = errors.New("managed component not built")

// Arguments returns the current arguments of the managed component.
func (cn *BuiltinComponentNode) Arguments() component.Arguments {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.args
}

// Block implements BlockNode and returns the current block of the managed component.
func (cn *BuiltinComponentNode) Block() *ast.BlockStmt {
	cn.mut.RLock()
	defer cn.mut.RUnlock()
	return cn.block
}

// Exports returns the current set of exports from the managed component.
// Exports returns nil if the managed component does not have exports.
func (cn *BuiltinComponentNode) Exports() component.Exports {
	cn.exportsMut.RLock()
	defer cn.exportsMut.RUnlock()
	return cn.exports
}

// setExports is called whenever the managed component updates. e must be the
// same type as the registered exports type of the managed component.
func (cn *BuiltinComponentNode) setExports(e component.Exports) {
	if cn.exportsType == nil {
		panic(fmt.Sprintf("Component %s called OnStateChange but never registered an Exports type", cn.nodeID))
	}
	if reflect.TypeOf(e) != cn.exportsType {
		panic(fmt.Sprintf("Component %s changed Exports types from %T to %T", cn.nodeID, cn.reg.Exports, e))
	}

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

// CurrentHealth returns the current health of the BuiltinComponentNode.
//
// The health of a BuiltinComponentNode is determined by combining:
//
//  1. Health from the call to Run().
//  2. Health from the last call to Evaluate().
//  3. Health reported from the component.
func (cn *BuiltinComponentNode) CurrentHealth() component.Health {
	cn.healthMut.RLock()
	defer cn.healthMut.RUnlock()

	var (
		runHealth  = cn.runHealth
		evalHealth = cn.evalHealth
	)

	if hc, ok := cn.managed.(component.HealthComponent); ok {
		componentHealth := hc.CurrentHealth()
		return component.LeastHealthy(runHealth, evalHealth, componentHealth)
	}

	return component.LeastHealthy(runHealth, evalHealth)
}

// DebugInfo returns debugging information from the managed component (if any).
func (cn *BuiltinComponentNode) DebugInfo() interface{} {
	cn.mut.RLock()
	defer cn.mut.RUnlock()

	if dc, ok := cn.managed.(component.DebugComponent); ok {
		return dc.DebugInfo()
	}
	return nil
}

// setEvalHealth sets the internal health from a call to Evaluate. See Health
// for information on how overall health is calculated.
func (cn *BuiltinComponentNode) setEvalHealth(t component.HealthType, msg string) {
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
func (cn *BuiltinComponentNode) setRunHealth(t component.HealthType, msg string) {
	cn.healthMut.Lock()
	defer cn.healthMut.Unlock()

	cn.runHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

// ModuleIDs returns the current list of modules that this component is
// managing.
func (cn *BuiltinComponentNode) ModuleIDs() []string {
	return cn.moduleController.ModuleIDs()
}
