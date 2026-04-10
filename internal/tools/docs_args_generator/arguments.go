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
	// SourceSchemaID is non-empty when this table was generated from a block
	// imported from a subschema (e.g. the net schema). It is used to route
	// the output file to the shared docs directory for that schema.
	SourceSchemaID string
	Rows           []ArgTableRow
}

// newArgumentsTables recursively builds argument tables for schema and all of
// its nested block properties. inheritedSourceID propagates the source schema
// ID of the nearest imported ancestor so that nested blocks are attributed to
// the same schema as their parent.
func newArgumentsTables(propName string, schema *jsonschema.Schema, inheritedSourceID string) []*ArgTable {
	var tables []*ArgTable

	if schema == nil {
		return tables
	}

	sourceID := schema.SourceID
	if sourceID == "" {
		sourceID = inheritedSourceID
	}

	curPropTable := &ArgTable{Name: propName, SourceSchemaID: sourceID}

	for name, prop := range schema.Properties {
		isPropRequired := slices.Contains(schema.Required, name)

		if prop.IsBlock() {
			childSourceID := prop.SourceID
			if childSourceID == "" {
				childSourceID = sourceID
			}
			propTables := newArgumentsTables(name, prop, childSourceID)
			tables = append(tables, propTables...)
			continue
		}

		curPropTable.Rows = append(curPropTable.Rows, ArgTableRow{
			Name:         name,
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
		// Required arguments come first; within each group sort alphabetically.
		iRequired := i.Required == requiredYes
		jRequired := j.Required == requiredYes
		if iRequired != jRequired {
			if iRequired {
				return -1
			}
			return 1
		}
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
