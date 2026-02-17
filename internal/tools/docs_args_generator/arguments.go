package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/tools/docs_args_generator/jsonschema"
)

type ArgTableRow struct {
	Name         string
	Type         string
	Description  string
	DefaultValue string
	Required     string
}

type ArgTable struct {
	Name string
	Rows []ArgTableRow
}

func newArgumentsTables(propName string, schema *jsonschema.Schema) []*ArgTable {
	var tables []*ArgTable

	curPropTable := &ArgTable{Name: propName}

	if schema == nil {
		return tables
	}

	for name, prop := range schema.Properties {
		isPropRequired := slices.Contains(schema.Required, name)

		if prop.IsBlock() {
			propTables := newArgumentsTables(name, prop)
			tables = append(tables, propTables...)
			continue
		}

		curPropTable.Rows = append(curPropTable.Rows, ArgTableRow{
			Name: name,
			// The names of toAlloyType and determineDefault should be consistent
			Type:         prop.ToAlloyType(),
			Description:  prop.Description,
			DefaultValue: prop.DetermineDefault(),
			Required:     printRequired(isPropRequired),
		})
	}

	tables = append(tables, curPropTable)
	return tables
}

func (t *ArgTable) sort() {
	slices.SortStableFunc(t.Rows, func(i, j ArgTableRow) int {
		// TODO: Sort by required arguments first
		return strings.Compare(i.Name, j.Name)
	})
}

func (t *ArgTable) markdown() string {
	if len(t.Rows) == 0 {
		return "There are no arguments."
	}

	var sb strings.Builder

	// Write table header
	sb.WriteString("| Name  | Type  | Description  | Default  | Required |\n")
	sb.WriteString("| ----- | ----- | ------------ | -------- | -------- |\n")

	// Process only argument properties (filter by alloy.io == "argument")
	for _, prop := range t.Rows {
		// Write table row
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s | %s |\n",
			// TODO: Fix the "required" later
			prop.Name, prop.Type, prop.Description, markdownCode(prop.DefaultValue), prop.Required))
	}

	return sb.String()
}

func markdownCode(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("`%s`", s)
}
