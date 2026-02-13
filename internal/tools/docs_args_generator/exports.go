package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/tools/docs_args_generator/jsonschema"
)

type ExportsTableRow struct {
	Name        string
	Type        string
	Description string
}

type ExportsTable struct {
	Rows []ExportsTableRow
}

func newExportsTable(s *jsonschema.Schema) *ExportsTable {
	table := ExportsTable{
		Rows: []ExportsTableRow{},
	}

	if s == nil {
		return &table
	}

	// Assume that there are no blocks in the exports
	for name, prop := range s.Properties {
		table.Rows = append(table.Rows, ExportsTableRow{
			Name:        name,
			Type:        prop.ToAlloyType(),
			Description: prop.Description,
		})
	}

	return &table
}

func (t *ExportsTable) sort() {
	slices.SortStableFunc(t.Rows, func(i, j ExportsTableRow) int {
		return strings.Compare(i.Name, j.Name)
	})
}

func (t *ExportsTable) markdown() (string, error) {
	if len(t.Rows) == 0 {
		return "There are no exports.", nil
	}

	var sb strings.Builder

	sb.WriteString("| Name | Type | Description |\n")
	sb.WriteString("| ---- | ---- | ----------- |\n")

	for _, prop := range t.Rows {
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
			prop.Name, prop.Type, prop.Description))
	}

	return sb.String(), nil
}
