package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/nodeconf/foreach"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

// The ForeachConfigNode will create the pipeline defined in its template block for each entry defined in its collection argument.
// Each pipeline is managed by a custom component.
// The custom component has access to the root scope (it can access exports and modules outside of the foreach template).
// The collection may contain any item. Each child has one item from the collection associated to him and that can be accessed via the defined var argument.
// Nesting foreach blocks is allowed.
type ForeachConfigNode struct {
	id               ComponentID
	nodeID           string
	label            string
	componentName    string
	moduleController ModuleController

	logger log.Logger

	// customReg is the customComponentRegistry of the current loader.
	// We pass it so that the foreach children have access to modules.
	customReg *CustomComponentRegistry

	customComponents          map[string]CustomComponent // track the children
	customComponentHashCounts map[string]int             // track the hash to avoid collisions

	forEachChildrenUpdateChan chan struct{} // used to trigger an update of the running children
	forEachChildrenRunning    bool

	mut   sync.RWMutex
	block *ast.BlockStmt
	args  foreach.Arguments

	moduleControllerFactory func(opts ModuleControllerOpts) ModuleController
	moduleControllerOpts    ModuleControllerOpts

	healthMut  sync.RWMutex
	evalHealth component.Health // Health of the last evaluate
	runHealth  component.Health // Health of running the component

	dataFlowEdgeMut  sync.RWMutex
	dataFlowEdgeRefs []string

	runner *runner.Runner[*forEachChild]
}

var _ ComponentNode = (*ForeachConfigNode)(nil)

func NewForeachConfigNode(block *ast.BlockStmt, globals ComponentGlobals, customReg *CustomComponentRegistry) *ForeachConfigNode {
	nodeID := BlockComponentID(block).String()
	globalID := nodeID
	if globals.ControllerID != "" {
		globalID = path.Join(globals.ControllerID, nodeID)
	}

	return &ForeachConfigNode{
		nodeID:                    nodeID,
		label:                     block.Label,
		block:                     block,
		componentName:             block.GetBlockName(),
		id:                        BlockComponentID(block),
		logger:                    log.With(globals.Logger, "component_path", globals.ControllerID, "component_id", nodeID),
		moduleControllerFactory:   globals.NewModuleController,
		moduleControllerOpts:      ModuleControllerOpts{Id: globalID},
		customReg:                 customReg,
		forEachChildrenUpdateChan: make(chan struct{}, 1),
		customComponents:          make(map[string]CustomComponent, 0),
		customComponentHashCounts: make(map[string]int, 0),
	}
}

func (fn *ForeachConfigNode) Label() string { return fn.label }

func (fn *ForeachConfigNode) NodeID() string { return fn.nodeID }

func (fn *ForeachConfigNode) Block() *ast.BlockStmt {
	fn.mut.RLock()
	defer fn.mut.RUnlock()
	return fn.block
}

func (fn *ForeachConfigNode) Arguments() component.Arguments {
	fn.mut.RLock()
	defer fn.mut.RUnlock()
	return fn.args
}

func (fn *ForeachConfigNode) ModuleIDs() []string {
	fn.mut.RLock()
	defer fn.mut.RUnlock()
	return fn.moduleController.ModuleIDs()
}

func (fn *ForeachConfigNode) ComponentName() string {
	return fn.componentName
}

// Exports returns nil as `foreach` doesn't have the ability to export values.
// This is something we could implement in the future if there is a need for it.
func (fn *ForeachConfigNode) Exports() component.Exports {
	return nil
}
func (fn *ForeachConfigNode) ID() ComponentID {
	return fn.id
}

func (fn *ForeachConfigNode) Evaluate(evalScope *vm.Scope) error {
	err := fn.evaluate(evalScope)

	switch err {
	case nil:
		fn.setEvalHealth(component.HealthTypeHealthy, "foreach evaluated")
	default:
		msg := fmt.Sprintf("foreach evaluation failed: %s", err)
		fn.setEvalHealth(component.HealthTypeUnhealthy, msg)
	}
	return err
}

func (fn *ForeachConfigNode) evaluate(scope *vm.Scope) error {
	fn.mut.Lock()
	defer fn.mut.Unlock()

	// Split the template block from the rest of the body because it should not be evaluated.
	var argsBody ast.Body
	var template *ast.BlockStmt
	for _, stmt := range fn.block.Body {
		if blockStmt, ok := stmt.(*ast.BlockStmt); ok && blockStmt.GetBlockName() == foreach.TypeTemplate {
			template = blockStmt
			continue
		}
		argsBody = append(argsBody, stmt)
	}

	if template == nil {
		return fmt.Errorf("the block template is missing in the foreach block")
	}

	eval := vm.New(argsBody)

	var args foreach.Arguments
	if err := eval.Evaluate(scope, &args); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}

	// By default don't show debug metrics.
	if args.EnableMetrics {
		// If metrics should be enabled, just use the regular registry.
		// There's no need to pass a special registry specific for this module controller.
		fn.moduleControllerOpts.RegOverride = nil
	} else {
		fn.moduleControllerOpts.RegOverride = NoopRegistry{}
	}

	if fn.moduleController == nil {
		fn.moduleController = fn.moduleControllerFactory(fn.moduleControllerOpts)
	} else if fn.args.EnableMetrics != args.EnableMetrics && fn.runner != nil {
		// When metrics are toggled on/off, we must recreate the module controller with the new registry.
		// This requires recreating and re-registering all components with the new controller.
		// Since enabling/disabling metrics is typically a one-time configuration change rather than
		// a frequent runtime toggle, the overhead of recreating components is acceptable.
		fn.moduleController = fn.moduleControllerFactory(fn.moduleControllerOpts)
		fn.customComponents = make(map[string]CustomComponent)
		err := fn.runner.ApplyTasks(context.Background(), []*forEachChild{}) // stops all running children
		if err != nil {
			return fmt.Errorf("error stopping foreach children: %w", err)
		}
	}

	fn.args = args

	// Loop through the items to create the custom components.
	// On re-evaluation new components are added and existing ones are updated.
	newCustomComponentIds := make(map[string]bool, len(args.Collection))
	fn.customComponentHashCounts = make(map[string]int)
	for i := 0; i < len(args.Collection); i++ {
		// Using default value for id as whole collection object
		id := args.Collection[i]

		// Extract Id from collection if exists
		if args.Id != "" {
			if val, ok := collectionItemID(args.Collection[i], args.Id, fn.logger); ok {
				// Use the field's value for fingerprinting
				id = val
			}
		}

		// We must create an ID from the collection entries to avoid recreating all components on every updates.
		// We track the hash counts because the collection might contain duplicates ([1, 1, 1] would result in the same ids
		// so we handle it by adding the count at the end -> [11, 12, 13]
		customComponentID := fmt.Sprintf("foreach_%s", objectFingerprint(id, args.HashStringId))
		count := fn.customComponentHashCounts[customComponentID] // count = 0 if the key is not found
		fn.customComponentHashCounts[customComponentID] = count + 1
		customComponentID += fmt.Sprintf("_%d", count+1)

		cc, created, err := fn.getOrCreateCustomComponent(customComponentID)
		if err != nil {
			return err
		}

		if created && args.HashStringId && id != nil && reflect.TypeOf(id).Kind() == reflect.String {
			level.Debug(fn.logger).Log("msg", "a new foreach pipeline was created", "value", id, "fingerprint", customComponentID)
		}

		// Expose the current scope + the collection item that correspond to the child.
		vars := deepCopyMap(scope.Variables)
		vars[args.Var] = args.Collection[i]

		customComponentRegistry := NewCustomComponentRegistry(fn.customReg, vm.NewScope(vars))
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

	// Trigger to stop previous children from running and to start running the new ones.
	if fn.forEachChildrenRunning {
		select {
		case fn.forEachChildrenUpdateChan <- struct{}{}: // queued trigger
		default: // trigger already queued; no-op
		}
	}
	return nil
}

// Assumes that a lock is held,
// so that fn.moduleController doesn't change while the function is running.
func (fn *ForeachConfigNode) getOrCreateCustomComponent(customComponentID string) (CustomComponent, bool, error) {
	cc, exists := fn.customComponents[customComponentID]
	if exists {
		return cc, false, nil
	}

	newCC, err := fn.moduleController.NewCustomComponent(customComponentID, func(exports map[string]any) {})
	if err != nil {
		return nil, true, fmt.Errorf("creating custom component: %w", err)
	}
	fn.customComponents[customComponentID] = newCC
	return newCC, true, nil
}

func (fn *ForeachConfigNode) UpdateBlock(b *ast.BlockStmt) {
	fn.mut.Lock()
	defer fn.mut.Unlock()
	fn.block = b
}

func (fn *ForeachConfigNode) Run(ctx context.Context) error {
	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	fn.runner = runner.New(func(forEachChild *forEachChild) runner.Worker {
		return &forEachChildRunner{
			child: forEachChild,
		}
	})
	defer fn.runner.Stop()

	updateTasks := func() error {
		fn.mut.Lock()
		defer fn.mut.Unlock()
		fn.forEachChildrenRunning = true
		var tasks []*forEachChild
		for customComponentID, customComponent := range fn.customComponents {
			tasks = append(tasks, &forEachChild{
				id:           customComponentID,
				cc:           customComponent,
				logger:       log.With(fn.logger, "foreach_path", fn.nodeID, "child_id", customComponentID),
				healthUpdate: fn.setRunHealth,
			})
		}
		return fn.runner.ApplyTasks(newCtx, tasks)
	}

	fn.setRunHealth(component.HealthTypeHealthy, "started foreach")

	err := updateTasks()
	if err != nil {
		return fmt.Errorf("running foreach children failed: %w", err)
	}

	fn.run(ctx, updateTasks)
	fn.setRunHealth(component.HealthTypeExited, "foreach node shut down cleanly")

	return nil
}

func (fn *ForeachConfigNode) run(ctx context.Context, updateTasks func() error) {
	for {
		select {
		case <-fn.forEachChildrenUpdateChan:
			err := updateTasks()
			if err != nil {
				level.Error(fn.logger).Log("msg", "error encountered while updating foreach children", "err", err)
				fn.setRunHealth(component.HealthTypeUnhealthy, fmt.Sprintf("error encountered while updating foreach children: %s", err))
				// the error is not fatal, the node can still run in unhealthy mode
			} else {
				fn.setRunHealth(component.HealthTypeHealthy, "foreach children updated successfully")
			}
		case <-ctx.Done():
			return
		}
	}
}

// CurrentHealth returns the current health of the ForeachConfigNode.
//
// The health of a ForeachConfigNode is determined by combining:
//
//  1. Health from the call to Run().
//  2. Health from the last call to Evaluate().
func (fn *ForeachConfigNode) CurrentHealth() component.Health {
	fn.healthMut.RLock()
	defer fn.healthMut.RUnlock()
	return component.LeastHealthy(fn.runHealth, fn.evalHealth)
}

func (fn *ForeachConfigNode) setEvalHealth(t component.HealthType, msg string) {
	fn.healthMut.Lock()
	defer fn.healthMut.Unlock()

	fn.evalHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

func (fn *ForeachConfigNode) setRunHealth(t component.HealthType, msg string) {
	fn.healthMut.Lock()
	defer fn.healthMut.Unlock()

	fn.runHealth = component.Health{
		Health:     t,
		Message:    msg,
		UpdateTime: time.Now(),
	}
}

func (fn *ForeachConfigNode) AddDataFlowEdgeTo(nodeID string) {
	fn.dataFlowEdgeMut.Lock()
	defer fn.dataFlowEdgeMut.Unlock()
	fn.dataFlowEdgeRefs = append(fn.dataFlowEdgeRefs, nodeID)
}

func (fn *ForeachConfigNode) GetDataFlowEdgesTo() []string {
	fn.dataFlowEdgeMut.RLock()
	defer fn.dataFlowEdgeMut.RUnlock()
	return fn.dataFlowEdgeRefs
}

func (fn *ForeachConfigNode) ResetDataFlowEdgeTo() {
	fn.dataFlowEdgeMut.Lock()
	defer fn.dataFlowEdgeMut.Unlock()
	fn.dataFlowEdgeRefs = []string{}
}

type forEachChildRunner struct {
	child *forEachChild
}

type forEachChild struct {
	cc           CustomComponent
	id           string
	logger       log.Logger
	healthUpdate func(t component.HealthType, msg string)
}

func (fr *forEachChildRunner) Run(ctx context.Context) {
	err := fr.child.cc.Run(ctx)
	if err != nil {
		level.Error(fr.child.logger).Log("msg", "foreach child stopped running", "err", err)
		fr.child.healthUpdate(component.HealthTypeUnhealthy, fmt.Sprintf("foreach child stopped running: %s", err))
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

// This function uses a 256 bits hash to minimize the risk of collisions between foreach children.
// If this is ever a performance bottleneck, it should still be totally safe to switch the 64bits hash.
func computeHash(s string) string {
	hasher := sha256.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}

func objectFingerprint(id any, hashId bool) string {
	// TODO: Test what happens if there is a "true" string and a true bool in the collection.
	switch v := id.(type) {
	case string:
		if hashId {
			return computeHash(v)
		}
		return replaceNonAlphaNumeric(v)
	case int, bool:
		return fmt.Sprintf("%v", v)
	case float64:
		// Dots are not valid characters in Alloy syntax identifiers.
		// For example, "foreach_3.14_1" should become "foreach_3_14_1".
		return strings.ReplaceAll(fmt.Sprintf("%f", v), ".", "_")
	default:
		return computeHash(fmt.Sprintf("%#v", v))
	}
}

func collectionItemID(item any, key string, logger log.Logger) (any, bool) {
	switch value := item.(type) {
	case map[string]any:
		// Inline object literals with simple values.
		// Example: collection = [{name = "one", port = "8080"}, {name = "two", port = "8081"}]
		val, ok := value[key]
		if !ok {
			logMissingCollectionID(logger, key)
			return nil, false
		}
		return val, true
	case map[string]string:
		// Plain Go maps - used to be common, but are now replaced by Target capsules for performance.
		// We keep it for maximum compatibility in case it's needed in the future.
		val, ok := value[key]
		if !ok {
			logMissingCollectionID(logger, key)
			return nil, false
		}
		return val, true
	case map[string]syntax.Value:
		// Inline object literals with expressions or computed values.
		// Example: collection = [{name = "one", url = "http://" + hostname}]
		val, ok := value[key]
		if !ok {
			logMissingCollectionID(logger, key)
			return nil, false
		}
		return val.Interface(), true
	case syntax.ConvertibleIntoCapsule:
		// Capsules from component exports, such as discovery.Target.
		// Example: collection = discovery.kubernetes.pods.targets
		return collectionItemIDFromCapsule(value, key, logger)
	default:
		level.Debug(logger).Log("msg", "unsupported collection item type encountered in foreach", "item", fmt.Sprintf("%#v", item))
		return nil, false
	}
}

func collectionItemIDFromCapsule(value syntax.ConvertibleIntoCapsule, key string, logger log.Logger) (any, bool) {
	var obj map[string]syntax.Value
	if err := value.ConvertInto(&obj); err == nil {
		val, ok := obj[key]
		if ok {
			return val.Interface(), true
		}
		logMissingCollectionID(logger, key)
		return nil, false
	}

	return nil, false
}

func logMissingCollectionID(logger log.Logger, key string) {
	level.Warn(logger).Log("msg", "specified id not found in collection item", "id", key)
}

func replaceNonAlphaNumeric(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

type NoopRegistry struct{}

var _ prometheus.Registerer = NoopRegistry{}

// MustRegister implements prometheus.Registerer.
func (n NoopRegistry) MustRegister(...prometheus.Collector) {}

// Register implements prometheus.Registerer.
func (n NoopRegistry) Register(prometheus.Collector) error {
	return nil
}

// Unregister implements prometheus.Registerer.
func (n NoopRegistry) Unregister(prometheus.Collector) bool {
	return true
}
