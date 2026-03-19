package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/tools/docs_args_generator/jsonschema"
)

type BlocksTableRow struct {
	Name           string
	ParentPath     []string
	Description    string
	Required       string
	SourceSchemaID string
	// DefName is set when the block was declared via an internal $ref (e.g.
	// "$ref: \"#/$defs/tls\""). It overrides the parent-path component of the
	// link ID so that every occurrence of the same $defs entry shares one link.
	DefName string
}

type BlocksTable struct {
	Rows []BlocksTableRow
}

// newBlocksTable recursively builds the blocks table for schema.
// inheritedSchemaID is the SourceID of the nearest ancestor block that was
// imported from a subschema; it propagates down so that nested blocks carry the
// same namespace as their top-level parent.
func newBlocksTable(parentBlocks []string, inheritedSchemaID string, schema *jsonschema.Schema) *BlocksTable {
	table := &BlocksTable{
		Rows: []BlocksTableRow{},
	}

	if schema == nil {
		return table
	}

	for name, prop := range schema.Properties {
		if !prop.IsBlock() {
			continue
		}

		isRequired := slices.Contains(schema.Required, name)

		// Use the block's own SourceID if it was directly imported, otherwise
		// inherit the ID from the parent block.
		sourceSchemaID := prop.SourceID
		if sourceSchemaID == "" {
			sourceSchemaID = inheritedSchemaID
		}

		table.Rows = append(table.Rows, BlocksTableRow{
			Name:           name,
			ParentPath:     append([]string{}, parentBlocks...),
			Description:    prop.Description,
			Required:       printRequired(isRequired),
			SourceSchemaID: sourceSchemaID,
			DefName:        prop.DefName,
		})

		newParentBlocks := append([]string{}, parentBlocks...)
		newParentBlocks = append(newParentBlocks, name)

		childTables := newBlocksTable(newParentBlocks, sourceSchemaID, prop)
		table.Rows = append(table.Rows, childTables.Rows...)
	}

	return table
}

// blockHierarchyChain returns the hierarchy display string for a row using the
// plain block name as the link target. Used for sorting.
func blockHierarchyChain(row BlocksTableRow) string {
	return printHierarchyWithLinkID(row.ParentPath, row.Name, row.Name)
}

// printHierarchyWithLinkID renders the hierarchy path for a block as a markdown
// reference. name is the displayed label; linkID is the link target identifier.
// Each parent segment is wrapped in backticks for consistent inline-code styling.
func printHierarchyWithLinkID(parentBlocks []string, name string, linkID string) string {
	ref := fmt.Sprintf("[`%s`][%s]", name, linkID)
	if len(parentBlocks) == 0 {
		return ref
	}
	parents := make([]string, len(parentBlocks))
	for i, p := range parentBlocks {
		parents[i] = fmt.Sprintf("`%s`", p)
	}
	return strings.Join(parents, " > ") + " > " + ref
}

func (t *BlocksTable) sort() {
	slices.SortStableFunc(t.Rows, func(i, j BlocksTableRow) int {
		// TODO: Sort by required blocks first
		return strings.Compare(blockHierarchyChain(i), blockHierarchyChain(j))
	})
}

// schemaSlug returns a short identifier derived from a schema ID by taking its
// last path segment. For example, "grafana/alloy/internal/component/common/net"
// becomes "net".
func schemaSlug(id string) string {
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}

// linkID returns the markdown link ID for the row.
//
// When a block was declared via an internal $ref (DefName is set), the def name
// is used instead of the full parent path so that every occurrence of the same
// $defs entry shares a single link (e.g. both "http > tls" and "grpc > tls"
// resolve to "net--tls" when they both $ref "#/$defs/tls").
//
// For blocks imported from a subschema without a $ref (SourceSchemaID is set),
// the schema slug prefixes the full parent path, e.g. "net--http--tls".
//
// For inline blocks (no SourceSchemaID), the full parent path is used, e.g.
// "grpc--tls". Top-level inline blocks with no parent use just their name.
func (row BlocksTableRow) linkID() string {
	if row.DefName != "" {
		if row.SourceSchemaID != "" {
			return schemaSlug(row.SourceSchemaID) + "--" + row.DefName
		}
		return row.DefName
	}

	parts := append([]string{}, row.ParentPath...)
	parts = append(parts, row.Name)
	pathID := strings.Join(parts, "--")

	if row.SourceSchemaID != "" {
		return schemaSlug(row.SourceSchemaID) + "--" + pathID
	}
	return pathID
}

func (t *BlocksTable) markdown() string {
	if len(t.Rows) == 0 {
		return "There are no blocks."
	}

	var sb strings.Builder

	sb.WriteString("| Block | Description | Required |\n")
	sb.WriteString("| ----- | ----------- | -------- |\n")

	for _, row := range t.Rows {
		chain := printHierarchyWithLinkID(row.ParentPath, row.Name, row.linkID())
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			chain, row.Description, row.Required))
	}

	sb.WriteString("\n")

	seen := make(map[string]bool)
	for _, row := range t.Rows {
		id := row.linkID()
		if !seen[id] {
			seen[id] = true
			sb.WriteString(fmt.Sprintf("[%s]: #%s\n", id, id))
		}
	}

	return sb.String()
}
