package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/grafana/alloy/internal/nodeconf/argument"
	"github.com/grafana/alloy/internal/nodeconf/export"
	"github.com/grafana/alloy/internal/nodeconf/foreach"
	"github.com/grafana/alloy/internal/nodeconf/importsource"
	"github.com/grafana/alloy/internal/static/config/encoder"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/parser"
)

// A Source holds the contents of a parsed Alloy configuration source module.
type Source struct {
	sourceMap map[string][]byte // Map that links parsed Alloy source's name with its content.
	fileMap   map[string]*ast.File

	// Components holds the list of raw Alloy AST blocks describing components.
	// The Alloy controller can interpret them.
	components    []*ast.BlockStmt
	configBlocks  []*ast.BlockStmt
	declareBlocks []*ast.BlockStmt
}

// ParseSource parses the Alloy file specified by bb into a File. name should be
// the name of the file used for reporting errors.
//
// bb must not be modified after passing to ParseSource.
func ParseSource(name string, bb []byte) (*Source, error) {
	bb, err := encoder.EnsureUTF8(bb, true)
	if err != nil {
		return nil, err
	}
	node, err := parser.ParseFile(name, bb)
	if err != nil {
		return nil, err
	}
	source, err := sourceFromBody(node.Body)
	if err != nil {
		return nil, err
	}
	source.sourceMap = map[string][]byte{name: bb}
	source.fileMap = map[string]*ast.File{name: node}
	return source, nil
}

// sourceFromBody creates a Source from an existing AST. This must only be used
// internally as there will be no sourceMap or hash.
func sourceFromBody(body ast.Body) (*Source, error) {
	// Look for predefined non-components blocks (i.e., logging), and store
	// everything else into a list of components.
	//
	// TODO(rfratto): should this code be brought into a helper somewhere? Maybe
	// in ast?
	var (
		components []*ast.BlockStmt
		configs    []*ast.BlockStmt
		declares   []*ast.BlockStmt
	)

	for _, stmt := range body {
		switch stmt := stmt.(type) {
		case *ast.AttributeStmt:
			return nil, diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(stmt.Name).Position(),
				EndPos:   ast.EndPos(stmt.Name).Position(),
				Message:  "unrecognized attribute " + stmt.Name.Name,
			}

		case *ast.BlockStmt:
			fullName := strings.Join(stmt.Name, ".")
			switch fullName {
			case "declare":
				declares = append(declares, stmt)
			case "logging", "tracing", argument.BlockName, export.BlockName, foreach.BlockName,
				importsource.BlockNameFile, importsource.BlockNameString, importsource.BlockNameHTTP, importsource.BlockNameGit:
				configs = append(configs, stmt)
			default:
				components = append(components, stmt)
			}

		default:
			return nil, diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(stmt).Position(),
				EndPos:   ast.EndPos(stmt).Position(),
				Message:  fmt.Sprintf("unsupported statement type %T", stmt),
			}
		}
	}

	return &Source{
		components:    components,
		configBlocks:  configs,
		declareBlocks: declares,
	}, nil
}

type namedSource struct {
	Name    string
	Content []byte
}

// ParseSources parses the map of sources and combines them into a single
// Source. sources must not be modified after calling ParseSources.
func ParseSources(sources map[string][]byte) (*Source, error) {
	var (
		// Collect diagnostic errors from several sources.
		mergedDiags diag.Diagnostics
		// Combined source from all the input content.
		mergedSource = &Source{
			sourceMap: sources,
			fileMap:   make(map[string]*ast.File, len(sources)),
		}
	)

	// Sorted slice so ParseSources always does the same thing.
	sortedSources := make([]namedSource, 0, len(sources))
	for name, bb := range sources {
		sortedSources = append(sortedSources, namedSource{
			Name:    name,
			Content: bb,
		})
	}
	sort.Slice(sortedSources, func(i, j int) bool {
		return sortedSources[i].Name < sortedSources[j].Name
	})

	// Parse each .alloy source and compute new hash for the whole sourceMap
	for _, namedSource := range sortedSources {
		sourceFragment, err := ParseSource(namedSource.Name, namedSource.Content)
		if err != nil {
			// If we encounter diagnostic errors we combine them and
			// later return all of them
			var diags diag.Diagnostics
			if errors.As(err, &diags) {
				mergedDiags = append(mergedDiags, diags...)
				continue
			}
			return nil, err
		}

		mergedSource.fileMap[namedSource.Name] = sourceFragment.fileMap[namedSource.Name]

		mergedSource.components = append(mergedSource.components, sourceFragment.components...)
		mergedSource.configBlocks = append(mergedSource.configBlocks, sourceFragment.configBlocks...)
		mergedSource.declareBlocks = append(mergedSource.declareBlocks, sourceFragment.declareBlocks...)
	}

	if len(mergedDiags) > 0 {
		return nil, mergedDiags
	}

	return mergedSource, nil
}

// RawConfigs returns the raw source content used to create Source.
// Do not modify the returned map.
func (s *Source) RawConfigs() map[string][]byte {
	if s == nil {
		return nil
	}
	return s.sourceMap
}

// SourceFiles returns the parsed source content used to create Source.
// Do not modify the returned map.
func (s *Source) SourceFiles() map[string]*ast.File {
	if s == nil {
		return nil
	}
	return s.fileMap
}

func (s *Source) Components() []*ast.BlockStmt {
	return s.components
}

func (s *Source) Configs() []*ast.BlockStmt {
	return s.configBlocks
}

func (s *Source) Declares() []*ast.BlockStmt {
	return s.declareBlocks
}
