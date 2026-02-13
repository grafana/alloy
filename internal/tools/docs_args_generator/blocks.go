package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/tools/docs_args_generator/jsonschema"
)

type BlocksTableRow struct {
	Name           string
	HierarchyChain string
	Description    string
	Required       string
}

type BlocksTable struct {
	Rows []BlocksTableRow
}

func newBlocksTable(parentBlocks []string, schema *jsonschema.Schema) *BlocksTable {
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

		table.Rows = append(table.Rows, BlocksTableRow{
			Name:           name,
			HierarchyChain: printHierarchy(parentBlocks, name),
			Description:    prop.Description,
			Required:       printRequired(isRequired),
		})

		newParentBlocks := append([]string{}, parentBlocks...)
		newParentBlocks = append(newParentBlocks, name)

		childTables := newBlocksTable(newParentBlocks, prop)
		table.Rows = append(table.Rows, childTables.Rows...)
	}

	return table
}

func printHierarchy(parentBlocks []string, name string) string {
	if len(parentBlocks) == 0 {
		return markdownLink(name).Reference
	}

	return strings.Join(parentBlocks, " > ") + " > " + markdownLink(name).Reference
}

func (t *BlocksTable) sort() {
	slices.SortStableFunc(t.Rows, func(i, j BlocksTableRow) int {
		// TODO: Sort by required blocks first
		return strings.Compare(i.HierarchyChain, j.HierarchyChain)
	})
}

func (t *BlocksTable) markdown() (string, error) {
	if len(t.Rows) == 0 {
		return "There are no blocks.", nil
	}

	var sb strings.Builder

	sb.WriteString("| Block | Description | Required |\n")
	sb.WriteString("| ----- | ----------- | -------- |\n")

	for _, prop := range t.Rows {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			prop.HierarchyChain, prop.Description, prop.Required))
	}

	sb.WriteString("\n")

	for _, prop := range t.Rows {
		// TODO: What if there are two blocks with the same name?
		// TODO: Search and replace various characters? I think markdown doesn't handle everything.
		sb.WriteString(fmt.Sprintf("[%s]: #%s\n", prop.Name, prop.Name))
	}

	return sb.String(), nil
}
