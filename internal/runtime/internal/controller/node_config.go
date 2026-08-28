package controller

import (
	"fmt"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/nodeconf/foreach"
	"github.com/grafana/alloy/internal/nodeconf/importsource"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
)

const (
	argumentBlockID = "argument"
	exportBlockID   = "export"
	loggingBlockID  = "logging"
	tracingBlockID  = "tracing"
)

// Add config blocks that are not GA. Config blocks that are not specified here are considered GA.
var configBlocksUnstable = map[string]featuregate.Stability{
	foreach.BlockName: foreach.StabilityLevel,
}

// NewConfigNode creates a new ConfigNode from an initial ast.BlockStmt.
// The underlying config isn't applied until Evaluate is called.
func NewConfigNode(block *ast.BlockStmt, globals ComponentGlobals, customReg *CustomComponentRegistry) (BlockNode, diag.Diagnostics) {
	var diags diag.Diagnostics

	if err := checkFeatureStability(block.GetBlockName(), globals.MinStability); err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  err.Error(),
			StartPos: ast.StartPos(block).Position(),
			EndPos:   ast.EndPos(block).Position(),
		})
		return nil, diags
	}

	switch block.GetBlockName() {
	case argumentBlockID:
		return NewArgumentConfigNode(block, globals), nil
	case exportBlockID:
		return NewExportConfigNode(block, globals), nil
	case loggingBlockID:
		return NewLoggingConfigNode(block, globals), nil
	case tracingBlockID:
		return NewTracingConfigNode(block, globals), nil
	case importsource.BlockNameFile, importsource.BlockNameString, importsource.BlockNameHTTP, importsource.BlockNameGit:
		return NewImportConfigNode(block, globals, importsource.GetSourceType(block.GetBlockName())), nil
	case foreach.BlockName:
		return NewForeachConfigNode(block, globals, customReg), nil
	default:
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("invalid config block type %s while creating new config node", block.GetBlockName()),
			StartPos: ast.StartPos(block).Position(),
			EndPos:   ast.EndPos(block).Position(),
		})
		return nil, diags
	}
}

func checkFeatureStability(blockName string, minStability featuregate.Stability) error {
	blockStability, exist := configBlocksUnstable[blockName]
	if exist {
		return featuregate.CheckAllowed(blockStability, minStability, fmt.Sprintf("config block %q", blockName))
	}
	return nil
}

// ConfigNodeMap represents the config BlockNodes in their explicit types.
// This is helpful when validating node conditions specific to config node
// types.
type ConfigNodeMap struct {
	logging     *LoggingConfigNode
	tracing     *TracingConfigNode
	argumentMap map[string]*ArgumentConfigNode
	exportMap   map[string]*ExportConfigNode
	importMap   map[string]*ImportConfigNode
	foreachMap  map[string]*ForeachConfigNode
}

// NewConfigNodeMap will create an initial ConfigNodeMap. Append must be called
// to populate NewConfigNodeMap.
func NewConfigNodeMap() *ConfigNodeMap {
	return &ConfigNodeMap{
		logging:     nil,
		tracing:     nil,
		argumentMap: map[string]*ArgumentConfigNode{},
		exportMap:   map[string]*ExportConfigNode{},
		importMap:   map[string]*ImportConfigNode{},
		foreachMap:  map[string]*ForeachConfigNode{},
	}
}

// Append will add a config node to the ConfigNodeMap. This will overwrite
// values on the ConfigNodeMap that are matched and previously set.
func (nodeMap *ConfigNodeMap) Append(configNode BlockNode) diag.Diagnostics {
	var diags diag.Diagnostics

	switch n := configNode.(type) {
	case *ArgumentConfigNode:
		nodeMap.argumentMap[n.Label()] = n
	case *ExportConfigNode:
		nodeMap.exportMap[n.Label()] = n
	case *LoggingConfigNode:
		nodeMap.logging = n
	case *TracingConfigNode:
		nodeMap.tracing = n
	case *ImportConfigNode:
		nodeMap.importMap[n.Label()] = n
	case *ForeachConfigNode:
		nodeMap.foreachMap[n.Label()] = n
	default:
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("unsupported config node type found %q", n.Block().Name),
			StartPos: ast.StartPos(n.Block()).Position(),
			EndPos:   ast.EndPos(n.Block()).Position(),
		})
	}

	return diags
}

// Validate wraps all validators for ConfigNodeMap.
func (nodeMap *ConfigNodeMap) Validate(isInModule bool, args map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics

	newDiags := nodeMap.ValidateModuleConstraints(isInModule)
	diags = append(diags, newDiags...)

	newDiags = nodeMap.ValidateUnsupportedArguments(args)
	diags = append(diags, newDiags...)

	return diags
}

// ValidateModuleConstraints will make sure config blocks with module
// constraints get followed.
func (nodeMap *ConfigNodeMap) ValidateModuleConstraints(isInModule bool) diag.Diagnostics {
	var diags diag.Diagnostics

	if isInModule {
		if nodeMap.logging != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  "logging block not allowed inside a module",
				StartPos: ast.StartPos(nodeMap.logging.Block()).Position(),
				EndPos:   ast.EndPos(nodeMap.logging.Block()).Position(),
			})
		}

		if nodeMap.tracing != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  "tracing block not allowed inside a module",
				StartPos: ast.StartPos(nodeMap.tracing.Block()).Position(),
				EndPos:   ast.EndPos(nodeMap.tracing.Block()).Position(),
			})
		}
		return diags
	}

	for key := range nodeMap.argumentMap {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  "argument blocks only allowed inside a module",
			StartPos: ast.StartPos(nodeMap.argumentMap[key].Block()).Position(),
			EndPos:   ast.EndPos(nodeMap.argumentMap[key].Block()).Position(),
		})
	}

	for key := range nodeMap.exportMap {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  "export blocks only allowed inside a module",
			StartPos: ast.StartPos(nodeMap.exportMap[key].Block()).Position(),
			EndPos:   ast.EndPos(nodeMap.exportMap[key].Block()).Position(),
		})
	}

	return diags
}

// ValidateUnsupportedArguments will validate each provided argument is
// supported in the config.
func (nodeMap *ConfigNodeMap) ValidateUnsupportedArguments(args map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics

	for argName := range args {
		if _, found := nodeMap.argumentMap[argName]; found {
			continue
		}
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("Provided argument %q is not defined in the module", argName),
		})
	}

	return diags
}
