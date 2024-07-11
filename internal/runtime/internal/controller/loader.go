package controller

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/dag"
	"github.com/grafana/alloy/internal/runtime/internal/worker"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/dskit/backoff"
	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// The Loader builds and evaluates ComponentNodes from Alloy blocks.
type Loader struct {
	log        log.Logger
	tracer     trace.TracerProvider
	globals    ComponentGlobals
	services   []service.Service
	host       service.Host
	workerPool worker.Pool
	// backoffConfig is used to backoff when an updated component's dependencies cannot be submitted to worker
	// pool for evaluation in EvaluateDependants, because the queue is full. This is an unlikely scenario, but when
	// it happens we should avoid retrying too often to give other goroutines a chance to progress. Having a backoff
	// also prevents log spamming with errors.
	backoffConfig backoff.Config

	mut                  sync.RWMutex
	graph                *dag.Graph
	originalGraph        *dag.Graph
	componentNodes       []ComponentNode
	declareNodes         map[string]*DeclareNode
	importConfigNodes    map[string]*ImportConfigNode
	serviceNodes         []*ServiceNode
	cache                *valueCache
	blocks               []*ast.BlockStmt // Most recently loaded blocks, used for writing
	cm                   *controllerMetrics
	cc                   *controllerCollector
	moduleExportIndex    int
	componentNodeManager *ComponentNodeManager
}

// LoaderOptions holds options for creating a Loader.
type LoaderOptions struct {
	// ComponentGlobals contains data to use when creating components.
	ComponentGlobals ComponentGlobals

	Services          []service.Service // Services to load into the DAG.
	Host              service.Host      // Service host (when running services).
	ComponentRegistry ComponentRegistry // Registry to search for components.
	WorkerPool        worker.Pool       // Worker pool to use for async tasks.
}

// NewLoader creates a new Loader. Components built by the Loader will be built
// with co for their options.
func NewLoader(opts LoaderOptions) *Loader {
	var (
		globals  = opts.ComponentGlobals
		services = opts.Services
		host     = opts.Host
		reg      = opts.ComponentRegistry
	)

	parent, id := splitPath(globals.ControllerID)

	if reg == nil {
		reg = NewDefaultComponentRegistry(opts.ComponentGlobals.MinStability, opts.ComponentGlobals.EnableCommunityComps)
	}

	l := &Loader{
		log:        log.With(globals.Logger, "controller_path", parent, "controller_id", id),
		tracer:     tracing.WrapTracerForLoader(globals.TraceProvider, globals.ControllerID),
		globals:    globals,
		services:   services,
		host:       host,
		workerPool: opts.WorkerPool,

		componentNodeManager: NewComponentNodeManager(globals, reg),

		// This is a reasonable default which should work for most cases. If a component is completely stuck, we would
		// retry and log an error every 10 seconds, at most.
		backoffConfig: backoff.Config{
			MinBackoff: 1 * time.Millisecond,
			MaxBackoff: 10 * time.Second,
		},

		graph:         &dag.Graph{},
		originalGraph: &dag.Graph{},
		cache:         newValueCache(),
		cm:            newControllerMetrics(parent, id),
	}
	l.cc = newControllerCollector(l, parent, id)

	if globals.Registerer != nil {
		globals.Registerer.MustRegister(l.cc)
		globals.Registerer.MustRegister(l.cm)
	}

	return l
}

// ApplyOptions are options that can be provided when loading a new Alloy config.
type ApplyOptions struct {
	Args map[string]any // input values of a module (nil for the root module)

	// TODO: rename ComponentBlocks because it also contains services
	ComponentBlocks []*ast.BlockStmt // pieces of config that can be used to instantiate builtin components and services
	ConfigBlocks    []*ast.BlockStmt // pieces of config that can be used to instantiate config nodes
	DeclareBlocks   []*ast.BlockStmt // pieces of config that can be used as templates to instantiate custom components

	// CustomComponentRegistry holds custom component templates.
	// The definition of a custom component instantiated inside of the loaded config
	// should be passed via this field if it's not declared or imported in the config.
	CustomComponentRegistry *CustomComponentRegistry
}

// Apply loads a new set of components into the Loader. Apply will drop any
// previously loaded component which is not described in the set of Alloy
// blocks.
//
// Apply will reuse existing components if there is an existing component which
// matches the component ID specified by any of the provided Alloy blocks.
// Reused components will be updated to point at the new Alloy block.
//
// Apply will perform an evaluation of all loaded components before returning.
// The provided parentContext can be used to provide global variables and
// functions to components. A child context will be constructed from the parent
// to expose values of other components.
func (l *Loader) Apply(options ApplyOptions) diag.Diagnostics {
	start := time.Now()
	l.mut.Lock()
	defer l.mut.Unlock()
	l.cm.controllerEvaluation.Set(1)
	defer l.cm.controllerEvaluation.Set(0)

	for key, value := range options.Args {
		l.cache.CacheModuleArgument(key, value)
	}
	l.cache.SyncModuleArgs(options.Args)

	// Create a new CustomComponentRegistry based on the provided one.
	// The provided one should be nil for the root config.
	l.componentNodeManager.setCustomComponentRegistry(NewCustomComponentRegistry(options.CustomComponentRegistry))
	newGraph, diags := l.loadNewGraph(options.Args, options.ComponentBlocks, options.ConfigBlocks, options.DeclareBlocks)
	if diags.HasErrors() {
		return diags
	}

	var (
		components   = make([]ComponentNode, 0)
		componentIDs = make([]ComponentID, 0)
		services     = make([]*ServiceNode, 0, len(l.services))
	)

	tracer := l.tracer.Tracer("")
	spanCtx, span := tracer.Start(context.Background(), "GraphEvaluate", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	logger := log.With(l.log, "trace_id", span.SpanContext().TraceID())
	level.Info(logger).Log("msg", "starting complete graph evaluation")
	defer func() {
		span.SetStatus(codes.Ok, "")

		level.Info(logger).Log("msg", "finished complete graph evaluation", "duration", time.Since(start))
	}()

	l.cache.ClearModuleExports()

	// Evaluate all the components.
	_ = dag.WalkTopological(&newGraph, newGraph.Leaves(), func(n dag.Node) error {
		_, span := tracer.Start(spanCtx, "EvaluateNode", trace.WithSpanKind(trace.SpanKindInternal))
		span.SetAttributes(attribute.String("node_id", n.NodeID()))
		defer span.End()

		start := time.Now()
		defer func() {
			level.Info(logger).Log("msg", "finished node evaluation", "node_id", n.NodeID(), "duration", time.Since(start))
		}()

		var err error

		switch n := n.(type) {
		case ComponentNode:
			components = append(components, n)
			componentIDs = append(componentIDs, n.ID())

			if err = l.evaluate(logger, n); err != nil {
				var evalDiags diag.Diagnostics
				if errors.As(err, &evalDiags) {
					diags = append(diags, evalDiags...)
				} else {
					diags.Add(diag.Diagnostic{
						Severity: diag.SeverityLevelError,
						Message:  fmt.Sprintf("Failed to build component: %s", err),
						StartPos: ast.StartPos(n.Block()).Position(),
						EndPos:   ast.EndPos(n.Block()).Position(),
					})
				}
			}

		case *ServiceNode:
			services = append(services, n)

			if err = l.evaluate(logger, n); err != nil {
				var evalDiags diag.Diagnostics
				if errors.As(err, &evalDiags) {
					diags = append(diags, evalDiags...)
				} else {
					diags.Add(diag.Diagnostic{
						Severity: diag.SeverityLevelError,
						Message:  fmt.Sprintf("Failed to evaluate service: %s", err),
						StartPos: ast.StartPos(n.Block()).Position(),
						EndPos:   ast.EndPos(n.Block()).Position(),
					})
				}
			}

		case BlockNode:
			if err = l.evaluate(logger, n); err != nil {
				diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					Message:  fmt.Sprintf("Failed to evaluate node for config block: %s", err),
					StartPos: ast.StartPos(n.Block()).Position(),
					EndPos:   ast.EndPos(n.Block()).Position(),
				})
			}
			if exp, ok := n.(*ExportConfigNode); ok {
				l.cache.CacheModuleExportValue(exp.Label(), exp.Value())
			}
		}

		// We only use the error for updating the span status; we don't return the
		// error because we want to evaluate as many nodes as we can.
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		return nil
	})

	l.componentNodes = components
	l.serviceNodes = services
	l.graph = &newGraph
	l.cache.SyncIDs(componentIDs)
	l.blocks = options.ComponentBlocks
	if l.globals.OnExportsChange != nil && l.cache.ExportChangeIndex() != l.moduleExportIndex {
		l.moduleExportIndex = l.cache.ExportChangeIndex()
		l.globals.OnExportsChange(l.cache.CreateModuleExports())
	}
	return diags
}

// Cleanup unregisters any existing metrics and optionally stops the worker pool.
func (l *Loader) Cleanup(stopWorkerPool bool) {
	if stopWorkerPool {
		l.workerPool.Stop()
	}
	if l.globals.Registerer == nil {
		return
	}
	l.globals.Registerer.Unregister(l.cm)
	l.globals.Registerer.Unregister(l.cc)
}

// loadNewGraph creates a new graph from the provided blocks and validates it.
func (l *Loader) loadNewGraph(args map[string]any, componentBlocks []*ast.BlockStmt, configBlocks []*ast.BlockStmt, declareBlocks []*ast.BlockStmt) (dag.Graph, diag.Diagnostics) {
	var g dag.Graph

	// Split component blocks into blocks for components and services.
	componentBlocks, serviceBlocks := l.splitComponentBlocks(componentBlocks)

	// Fill our graph with service blocks, which must be added before any other
	// block.
	diags := l.populateServiceNodes(&g, serviceBlocks)

	// Fill our graph with declare blocks, must be added before componentNodes.
	declareDiags := l.populateDeclareNodes(&g, declareBlocks)
	diags = append(diags, declareDiags...)

	// Fill our graph with config blocks.
	configBlockDiags := l.populateConfigBlockNodes(args, &g, configBlocks)
	diags = append(diags, configBlockDiags...)

	// Fill our graph with components.
	componentNodeDiags := l.populateComponentNodes(&g, componentBlocks)
	diags = append(diags, componentNodeDiags...)

	// Write up the edges of the graph
	wireDiags := l.wireGraphEdges(&g)
	diags = append(diags, wireDiags...)

	// Validate graph to detect cycles
	err := dag.Validate(&g)
	if err != nil {
		diags = append(diags, multierrToDiags(err)...)
		return g, diags
	}

	// Copy the original graph, this is so we can have access to the original graph for things like displaying a UI or
	// debug information.
	l.originalGraph = g.Clone()
	// Perform a transitive reduction of the graph to clean it up.
	dag.Reduce(&g)

	return g, diags
}

func (l *Loader) splitComponentBlocks(blocks []*ast.BlockStmt) (componentBlocks, serviceBlocks []*ast.BlockStmt) {
	componentBlocks = make([]*ast.BlockStmt, 0, len(blocks))
	serviceBlocks = make([]*ast.BlockStmt, 0, len(l.services))

	serviceNames := make(map[string]struct{}, len(l.services))
	for _, svc := range l.services {
		serviceNames[svc.Definition().Name] = struct{}{}
	}

	for _, block := range blocks {
		if _, isService := serviceNames[BlockComponentID(block).String()]; isService {
			serviceBlocks = append(serviceBlocks, block)
		} else {
			componentBlocks = append(componentBlocks, block)
		}
	}

	return componentBlocks, serviceBlocks
}

func (l *Loader) populateDeclareNodes(g *dag.Graph, declareBlocks []*ast.BlockStmt) diag.Diagnostics {
	var (
		diags    diag.Diagnostics
		node     *DeclareNode
		blockMap = make(map[string]*ast.BlockStmt, len(declareBlocks))
	)
	l.declareNodes = map[string]*DeclareNode{}
	for _, declareBlock := range declareBlocks {
		id := BlockComponentID(declareBlock).String()

		if declareBlock.Label == declareType {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  "'declare' is not a valid label for a declare block",
				StartPos: ast.StartPos(declareBlock).Position(),
				EndPos:   ast.EndPos(declareBlock).Position(),
			})
			continue
		}

		if diag, defined := blockAlreadyDefined(blockMap, id, declareBlock); defined {
			diags = append(diags, diag)
			continue
		}

		if exist := l.graph.GetByID(id); exist != nil {
			node = exist.(*DeclareNode)
			node.UpdateBlock(declareBlock)
		} else {
			node = NewDeclareNode(declareBlock)
		}
		l.componentNodeManager.customComponentReg.registerDeclare(declareBlock)
		l.declareNodes[node.label] = node
		g.Add(node)
	}
	return diags
}

// blockAlreadyDefined returns (diag, true) if the given id is already in the provided blockMap.
// else it adds the block to the map and returns (empty diag, false).
func blockAlreadyDefined(blockMap map[string]*ast.BlockStmt, id string, block *ast.BlockStmt) (diag.Diagnostic, bool) {
	if orig, redefined := blockMap[id]; redefined {
		return diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("block %s already declared at %s", id, ast.StartPos(orig).Position()),
			StartPos: block.NamePos.Position(),
			EndPos:   block.NamePos.Add(len(id) - 1).Position(),
		}, true
	}
	blockMap[id] = block
	return diag.Diagnostic{}, false
}

// populateServiceNodes adds service nodes to the graph.
func (l *Loader) populateServiceNodes(g *dag.Graph, serviceBlocks []*ast.BlockStmt) diag.Diagnostics {
	var diags diag.Diagnostics

	// First, build the services.
	for _, svc := range l.services {
		if !l.isRootController() {
			break
		}

		id := svc.Definition().Name

		if g.GetByID(id) != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  fmt.Sprintf("cannot add service %q; node with same ID already exists", id),
			})

			continue
		}

		var node *ServiceNode

		// Check the graph from the previous call to Load to see we can copy an
		// existing instance of ServiceNode.
		if exist := l.graph.GetByID(id); exist != nil {
			node = exist.(*ServiceNode)
		} else {
			node = NewServiceNode(l.host, svc)
		}

		node.UpdateBlock(nil) // Reset configuration to nil.
		g.Add(node)
	}

	// Now, assign blocks to services.
	for _, block := range serviceBlocks {
		blockID := BlockComponentID(block).String()

		if !l.isRootController() {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  fmt.Sprintf("service blocks not allowed inside a module: %q", blockID),
				StartPos: ast.StartPos(block).Position(),
				EndPos:   ast.EndPos(block).Position(),
			})
			continue
		}

		node := g.GetByID(blockID).(*ServiceNode)

		// Don't permit configuring services that have a lower stability level than
		// what is currently enabled.
		nodeStability := node.Service().Definition().Stability
		if err := featuregate.CheckAllowed(nodeStability, l.globals.MinStability, fmt.Sprintf("block %q", blockID)); err != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  err.Error(),
				StartPos: ast.StartPos(block).Position(),
				EndPos:   ast.EndPos(block).Position(),
			})
			continue
		}

		// Blocks assigned to services are reset to nil in the previous loop.
		//
		// If the block is non-nil, it means that there was a duplicate block
		// configuring the same service found in a previous iteration of this loop.
		if node.Block() != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  fmt.Sprintf("duplicate definition of %q", blockID),
				StartPos: ast.StartPos(block).Position(),
				EndPos:   ast.EndPos(block).Position(),
			})
			continue
		}

		node.UpdateBlock(block)
	}

	return diags
}

// populateConfigBlockNodes adds any config blocks to the graph.
func (l *Loader) populateConfigBlockNodes(args map[string]any, g *dag.Graph, configBlocks []*ast.BlockStmt) diag.Diagnostics {
	var (
		diags    diag.Diagnostics
		nodeMap  = NewConfigNodeMap()
		blockMap = make(map[string]*ast.BlockStmt, len(configBlocks))
	)

	for _, block := range configBlocks {
		var (
			node               BlockNode
			newConfigNodeDiags diag.Diagnostics
		)
		id := BlockComponentID(block).String()
		if diag, defined := blockAlreadyDefined(blockMap, id, block); defined {
			diags = append(diags, diag)
			continue
		}
		// Check the graph from the previous call to Load to see we can copy an
		// existing instance of BlockNode.
		if exist := l.graph.GetByID(id); exist != nil {
			node = exist.(BlockNode)
			node.UpdateBlock(block)
		} else {
			node, newConfigNodeDiags = NewConfigNode(block, l.globals)
			diags = append(diags, newConfigNodeDiags...)
			if diags.HasErrors() {
				continue
			}
		}

		nodeMapDiags := nodeMap.Append(node)
		diags = append(diags, nodeMapDiags...)
		if diags.HasErrors() {
			continue
		}

		if importNode, ok := node.(*ImportConfigNode); ok {
			l.componentNodeManager.customComponentReg.registerImport(importNode.label)
		}

		g.Add(node)
	}

	validateDiags := nodeMap.Validate(!l.isRootController(), args)
	diags = append(diags, validateDiags...)

	// If a logging config block is not provided, we create an empty node which uses defaults.
	if nodeMap.logging == nil && l.isRootController() {
		c := NewDefaultLoggingConfigNode(l.globals)
		g.Add(c)
	}

	// If a tracing config block is not provided, we create an empty node which uses defaults.
	if nodeMap.tracing == nil && l.isRootController() {
		c := NewDefaulTracingConfigNode(l.globals)
		g.Add(c)
	}

	l.importConfigNodes = nodeMap.importMap

	return diags
}

// populateComponentNodes adds any components to the graph.
func (l *Loader) populateComponentNodes(g *dag.Graph, componentBlocks []*ast.BlockStmt) diag.Diagnostics {
	var (
		diags    diag.Diagnostics
		blockMap = make(map[string]*ast.BlockStmt, len(componentBlocks))
	)
	for _, block := range componentBlocks {
		id := BlockComponentID(block).String()
		if diag, defined := blockAlreadyDefined(blockMap, id, block); defined {
			diags = append(diags, diag)
			continue
		}
		// Check the graph from the previous call to Load to see if we can copy an
		// existing instance of ComponentNode.
		if exist := l.graph.GetByID(id); exist != nil {
			c := exist.(ComponentNode)
			c.UpdateBlock(block)
			g.Add(c)
		} else {
			componentName := block.GetBlockName()
			c, err := l.componentNodeManager.createComponentNode(componentName, block)
			if err != nil {
				diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					Message:  err.Error(),
					StartPos: block.NamePos.Position(),
					EndPos:   block.NamePos.Add(len(componentName) - 1).Position(),
				})
				continue
			}
			g.Add(c)
		}
	}

	return diags
}

// Wire up all the related nodes
func (l *Loader) wireGraphEdges(g *dag.Graph) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, n := range g.Nodes() {
		switch n := n.(type) {
		case *ServiceNode: // Service depending on other services.
			for _, depName := range n.Definition().DependsOn {
				dep := g.GetByID(depName)
				if dep == nil {
					diags.Add(diag.Diagnostic{
						Severity: diag.SeverityLevelError,
						Message:  fmt.Sprintf("service %q has invalid reference to service %q", n.NodeID(), depName),
					})
					continue
				}

				g.AddEdge(dag.Edge{From: n, To: dep})
			}
		case *DeclareNode:
			// Although they do nothing on evaluation, DeclareNodes are wired
			// to detect cyclic dependencies. If a declare "a" block contains an instance
			// of a declare "b" which contains an instance of the declare "a", both DeclareNodes
			// will depend on each others, creating a cycle in the graph which will be detected later.
			// Example: declare "a"{b "default"{}} declare "b"{a "default"{}}
			// It also covers self-dependency.
			// Example: declare "a"{a "default"{}}
			refs := l.findCustomComponentReferences(n.Block())
			for ref := range refs {
				g.AddEdge(dag.Edge{From: n, To: ref})
			}
			// skip here because for now Declare nodes can't reference component nodes.
			continue
		case *CustomComponentNode:
			l.wireCustomComponentNode(g, n)
		}

		// Finally, wire component references.
		refs, nodeDiags := ComponentReferences(n, g)
		for _, ref := range refs {
			g.AddEdge(dag.Edge{From: n, To: ref.Target})
		}
		diags = append(diags, nodeDiags...)
	}

	return diags
}

// wireCustomComponentNode wires a custom component to the import/declare nodes that it depends on.
func (l *Loader) wireCustomComponentNode(g *dag.Graph, cc *CustomComponentNode) {
	// It's important to check first if the importNamespace matches an import node because there might be a
	// local node that has the same label as an imported declare.
	if importNode, ok := l.importConfigNodes[cc.importNamespace]; ok {
		// add an edge between the custom component and the corresponding import node.
		g.AddEdge(dag.Edge{From: cc, To: importNode})
	} else if declare, ok := l.declareNodes[cc.customComponentName]; ok {
		refs := l.findCustomComponentReferences(declare.Block())
		for ref := range refs {
			// add edges between the custom component and declare/import nodes.
			g.AddEdge(dag.Edge{From: cc, To: ref})
		}
	}
}

// Variables returns the Variables the Loader exposes for other components to
// reference.
func (l *Loader) Variables() map[string]interface{} {
	return l.cache.BuildContext().Variables
}

// Components returns the current set of loaded components.
func (l *Loader) Components() []ComponentNode {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.componentNodes
}

// Services returns the current set of service nodes.
func (l *Loader) Services() []*ServiceNode {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.serviceNodes
}

// Imports returns the current set of import nodes.
func (l *Loader) Imports() map[string]*ImportConfigNode {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.importConfigNodes
}

// Graph returns a copy of the DAG managed by the Loader.
func (l *Loader) Graph() *dag.Graph {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.graph.Clone()
}

// OriginalGraph returns a copy of the graph before Reduce was called. This can be used if you want to show a UI of the
// original graph before the reduce function was called.
func (l *Loader) OriginalGraph() *dag.Graph {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.originalGraph.Clone()
}

// EvaluateDependants sends nodes which depend directly on nodes in updatedNodes for evaluation to the
// workerPool. It should be called whenever nodes update their exports.
// It is beneficial to call EvaluateDependants with a batch of nodes, as it will enqueue the entire batch before
// the worker pool starts to evaluate them, resulting in smaller number of total evaluations when
// node updates are frequent. If the worker pool's queue is full, EvaluateDependants will retry with a backoff until
// it succeeds or until the ctx is cancelled.
func (l *Loader) EvaluateDependants(ctx context.Context, updatedNodes []*QueuedNode) {
	if len(updatedNodes) == 0 {
		return
	}
	tracer := l.tracer.Tracer("")
	spanCtx, span := tracer.Start(context.Background(), "SubmitDependantsForEvaluation", trace.WithSpanKind(trace.SpanKindInternal))
	span.SetAttributes(attribute.Int("originators_count", len(updatedNodes)))
	span.SetStatus(codes.Ok, "dependencies submitted for evaluation")
	defer span.End()

	l.cm.controllerEvaluation.Set(1)
	defer l.cm.controllerEvaluation.Set(0)

	l.mut.RLock()
	defer l.mut.RUnlock()

	dependenciesToParentsMap := make(map[dag.Node]*QueuedNode)
	for _, parent := range updatedNodes {
		switch parentNode := parent.Node.(type) {
		case ComponentNode:
			// Make sure we're in-sync with the current exports of parent.
			l.cache.CacheExports(parentNode.ID(), parentNode.Exports())
		case *ImportConfigNode:
			// Update the scope with the imported content.
			l.componentNodeManager.customComponentReg.updateImportContent(parentNode)
		}
		// We collect all nodes directly incoming to parent.
		_ = dag.WalkIncomingNodes(l.graph, parent.Node, func(n dag.Node) error {
			dependenciesToParentsMap[n] = parent
			return nil
		})
	}

	// Submit all dependencies for asynchronous evaluation.
	// During evaluation, if a node's exports change, Alloy will add it to updated nodes queue (controller.Queue) and
	// the Alloy controller will call EvaluateDependants on it again. This results in a concurrent breadth-first
	// traversal of the nodes that need to be evaluated.
	for n, parent := range dependenciesToParentsMap {
		dependantCtx, span := tracer.Start(spanCtx, "SubmitForEvaluation", trace.WithSpanKind(trace.SpanKindInternal))
		span.SetAttributes(attribute.String("node_id", n.NodeID()))
		span.SetAttributes(attribute.String("originator_id", parent.Node.NodeID()))

		// Submit for asynchronous evaluation with retries and backoff. Don't use range variables in the closure.
		var (
			nodeRef, parentRef = n, parent
			retryBackoff       = backoff.New(ctx, l.backoffConfig)
			err                error
		)
		for retryBackoff.Ongoing() {
			globalUniqueKey := path.Join(l.globals.ControllerID, nodeRef.NodeID())
			err = l.workerPool.SubmitWithKey(globalUniqueKey, func() {
				l.concurrentEvalFn(nodeRef, dependantCtx, tracer, parentRef)
			})
			if err != nil {
				level.Error(l.log).Log(
					"msg", "failed to submit node for evaluation - Alloy is likely overloaded "+
						"and cannot keep up with evaluating components - will retry",
					"err", err,
					"node_id", n.NodeID(),
					"originator_id", parent.Node.NodeID(),
					"retries", retryBackoff.NumRetries(),
				)
				retryBackoff.Wait()
			} else {
				break
			}
		}
		span.SetAttributes(attribute.Int("retries", retryBackoff.NumRetries()))
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "node submitted for evaluation")
		}
		span.End()
	}

	// Report queue size metric.
	l.cm.evaluationQueueSize.Set(float64(l.workerPool.QueueSize()))
}

// concurrentEvalFn returns a function that evaluates a node and updates the cache. This function can be submitted to
// a worker pool for asynchronous evaluation.
func (l *Loader) concurrentEvalFn(n dag.Node, spanCtx context.Context, tracer trace.Tracer, parent *QueuedNode) {
	start := time.Now()
	l.cm.dependenciesWaitTime.Observe(time.Since(parent.LastUpdatedTime).Seconds())
	_, span := tracer.Start(spanCtx, "EvaluateNode", trace.WithSpanKind(trace.SpanKindInternal))
	span.SetAttributes(attribute.String("node_id", n.NodeID()))
	defer span.End()

	defer func() {
		duration := time.Since(start)
		l.cm.onComponentEvaluationDone(n.NodeID(), duration)
		level.Debug(l.log).Log("msg", "finished node evaluation", "node_id", n.NodeID(), "duration", duration)
	}()

	var err error
	switch n := n.(type) {
	case BlockNode:
		ectx := l.cache.BuildContext()

		// RLock before evaluate to prevent Evaluating while the config is being reloaded
		l.mut.RLock()
		evalErr := n.Evaluate(ectx)

		err = l.postEvaluate(l.log, n, evalErr)

		// Additional post-evaluation steps necessary for module exports.
		if exp, ok := n.(*ExportConfigNode); ok {
			l.cache.CacheModuleExportValue(exp.Label(), exp.Value())
		}
		if l.globals.OnExportsChange != nil && l.cache.ExportChangeIndex() != l.moduleExportIndex {
			// Upgrade to write lock to update the module exports.
			l.mut.RUnlock()
			l.mut.Lock()
			defer l.mut.Unlock()
			// Check if the update still needed after obtaining the write lock and perform it.
			if l.cache.ExportChangeIndex() != l.moduleExportIndex {
				l.globals.OnExportsChange(l.cache.CreateModuleExports())
				l.moduleExportIndex = l.cache.ExportChangeIndex()
			}
		} else {
			// No need to upgrade to write lock, just release the read lock.
			l.mut.RUnlock()
		}
	}

	// We only use the error for updating the span status
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "node successfully evaluated")
	}
}

// evaluate constructs the final context for the BlockNode and
// evaluates it. mut must be held when calling evaluate.
func (l *Loader) evaluate(logger log.Logger, bn BlockNode) error {
	ectx := l.cache.BuildContext()
	err := bn.Evaluate(ectx)
	return l.postEvaluate(logger, bn, err)
}

// postEvaluate is called after a node has been evaluated. It updates the caches and logs any errors.
// mut must be held when calling postEvaluate.
func (l *Loader) postEvaluate(logger log.Logger, bn BlockNode, err error) error {
	switch c := bn.(type) {
	case ComponentNode:
		// Always update the cache both the arguments and exports, since both might
		// change when a component gets re-evaluated. We also want to cache the arguments and exports in case of an error
		l.cache.CacheArguments(c.ID(), c.Arguments())
		l.cache.CacheExports(c.ID(), c.Exports())
	case *ArgumentConfigNode:
		if _, found := l.cache.moduleArguments[c.Label()]; !found {
			if c.Optional() {
				l.cache.CacheModuleArgument(c.Label(), c.Default())
			} else {
				// NOTE: this masks the previous evaluation error, but we treat a missing module arguments as
				// a more important error to address.
				err = fmt.Errorf("missing required argument %q to module", c.Label())
			}
		}
	case *ImportConfigNode:
		l.componentNodeManager.customComponentReg.updateImportContent(c)
	}

	if err != nil {
		level.Error(logger).Log("msg", "failed to evaluate config", "node", bn.NodeID(), "err", err)
		return err
	}
	return nil
}

func multierrToDiags(errors error) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, err := range errors.(*multierror.Error).Errors {
		// TODO(rfratto): should this include position information?
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  err.Error(),
		})
	}
	return diags
}

// isRootController returns true if the loader is for the root Alloy controller.
func (l *Loader) isRootController() bool {
	return l.globals.ControllerID == ""
}

// findCustomComponentReferences returns references to import/declare nodes in a declare block.
func (l *Loader) findCustomComponentReferences(declare *ast.BlockStmt) map[BlockNode]struct{} {
	uniqueReferences := make(map[BlockNode]struct{})
	l.collectCustomComponentReferences(declare.Body, uniqueReferences)
	return uniqueReferences
}

// collectCustomComponentDependencies recursively collects references to import/declare nodes through an AST body.
func (l *Loader) collectCustomComponentReferences(stmts ast.Body, uniqueReferences map[BlockNode]struct{}) {
	for _, stmt := range stmts {
		blockStmt, ok := stmt.(*ast.BlockStmt)
		if !ok {
			continue
		}

		var (
			componentName = strings.Join(blockStmt.Name, ".")

			declareNode, foundDeclare = l.declareNodes[blockStmt.Name[0]]
			importNode, foundImport   = l.importConfigNodes[blockStmt.Name[0]]
		)

		switch {
		case componentName == declareType:
			l.collectCustomComponentReferences(blockStmt.Body, uniqueReferences)
		case foundDeclare:
			uniqueReferences[declareNode] = struct{}{}
		case foundImport:
			uniqueReferences[importNode] = struct{}{}
		}
	}
}

func splitPath(id string) (string, string) {
	parent, id := path.Split(id)
	parent, _ = strings.CutSuffix(parent, "/")
	return "/" + parent, id
}
