package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/tools/alloygen/metadata"
	"github.com/grafana/alloy/internal/tools/alloygen/validate"
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

type BlocksTableRow struct {
	Name           string
	HierarchyChain string
	Description    string
	Required       string
}

type BlocksTable struct {
	Rows []BlocksTableRow
}

type ExportsTableRow struct {
	Name        string
	Type        string
	Description string
}

type ExportsTable struct {
	Rows []ExportsTableRow
}

func main() {
	cmd, err := NewCommand()
	cobra.CheckErr(err)
	cobra.CheckErr(cmd.Execute())
}

func NewCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:          "mdatagen",
		Version:      "0.0.1",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	rootCmd.AddCommand(&cobra.Command{
		Use: "generate",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOnce(args[0], args[1])
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use: "validate",
		RunE: func(cmd *cobra.Command, args []string) error {
			return validate.Run(args)
		},
	})

	return rootCmd, nil
}

func runOnce(ymlPath string, outputPath string) error {
	log.Printf("Processing YAML schema: %s", ymlPath)
	log.Printf("Output directory: %s", outputPath)

	// Check if the YAML file exists
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		return nil // File doesn't exist, return early
	} else if err != nil {
		return err // Some other error occurred
	}

	// Parse the YAML schema
	schema, err := metadata.FromPath(ymlPath)
	if err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	ymlPathDir := filepath.Dir(ymlPath)

	err = mergeSubschemas(ymlPathDir, schema.Arguments)
	if err != nil {
		return fmt.Errorf("failed to merge argument subschemas: %w", err)
	}
	err = mergeSubschemas(ymlPathDir, schema.Exports)
	if err != nil {
		return fmt.Errorf("failed to merge export subschemas: %w", err)
	}

	argumentsTables := generateArgumentsTables("__arguments", schema.Arguments)
	blocksTable := generateBlocksTable([]string{}, schema.Arguments)
	exportsTable := generateExportsTable(schema.Exports)

	sortArgTables(argumentsTables)
	sortBlocksTables(blocksTable)
	sortExportsTables(exportsTable)

	// Generate the reference table
	markdownTables, err := generateMarkdownTables(argumentsTables, blocksTable, exportsTable)
	if err != nil {
		return fmt.Errorf("failed to generate reference table: %w", err)
	}

	writeMarkdownTables(markdownTables, outputPath)
	if err != nil {
		return fmt.Errorf("failed to write markdown tables: %w", err)
	}

	return nil
}

func mergeSubschemas(ymlPathDir string, schema *metadata.Schema) error {
	if schema == nil {
		return nil
	}

	// Merge allOf properties
	for _, prop := range schema.AllOf {
		// TODO: Support refs which are not files
		if prop.Ref != "" {
			// Load the referenced schema
			referencePath := filepath.Join(ymlPathDir, prop.Ref)
			log.Printf("Processing YAML subschema: %s", referencePath)

			parsedProp, err := metadata.FromPath2(referencePath)
			if err != nil {
				return fmt.Errorf("failed to parse schema file: %w", err)
			}

			// Add the properties from nested schema
			err = mergeSubschemas(ymlPathDir, parsedProp)
			if err != nil {
				return err
			}

			// Add the properties from the referenced schema
			for name, prop := range parsedProp.Definitions {
				// TODO: Copy the prop?
				schema.Properties[name] = prop
			}
		}
	}

	return nil
}

func writeMarkdownTables(markdownTables map[string]string, outputPath string) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}

	for name, table := range markdownTables {
		textFilePath := filepath.Join(outputPath, name+".md")
		file, err := os.Create(textFilePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = file.WriteString(table)
		if err != nil {
			return err
		}
	}

	return nil
}

func generateBlocksTable(parentBlocks []string, schema *metadata.Schema) BlocksTable {
	table := BlocksTable{
		Rows: []BlocksTableRow{},
	}

	if schema == nil {
		return table
	}

	for name, prop := range schema.Properties {
		if !isBlock(prop) {
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

		childTables := generateBlocksTable(newParentBlocks, prop)
		table.Rows = append(table.Rows, childTables.Rows...)
	}

	return table
}

func generateExportsTable(schema *metadata.Schema) ExportsTable {
	table := ExportsTable{
		Rows: []ExportsTableRow{},
	}

	if schema == nil {
		return table
	}

	// Assume that there are no blocks in the exports
	for name, prop := range schema.Properties {
		table.Rows = append(table.Rows, ExportsTableRow{
			Name:        name,
			Type:        toAlloyType(prop),
			Description: prop.Description,
		})
	}

	return table
}

func printHierarchy(parentBlocks []string, name string) string {
	if len(parentBlocks) == 0 {
		return markdownLink(name).Reference
	}

	return strings.Join(parentBlocks, " > ") + " > " + markdownLink(name).Reference
}

type MarkdownLink struct {
	Definition string // For example: [header]: #header
	Reference  string // For example: [`header`][header]
}

func markdownLink(name string) MarkdownLink {
	return MarkdownLink{
		Definition: fmt.Sprintf("[%s](#%s)", name, name),
		Reference:  fmt.Sprintf("[`%s`][%s]", name, name),
	}
}

func generateArgumentsTables(propName string, schema *metadata.Schema) []ArgTable {
	var tables []ArgTable

	curPropTable := ArgTable{
		Name: propName,
	}

	if schema == nil {
		return tables
	}

	for name, prop := range schema.Properties {
		isPropRequired := slices.Contains(schema.Required, name)

		if isBlock(prop) {
			propTables := generateArgumentsTables(name, prop)
			tables = append(tables, propTables...)
			continue
		}

		curPropTable.Rows = append(curPropTable.Rows, ArgTableRow{
			Name: name,
			// The names of toAlloyType and determineDefault should be consistent
			Type:         toAlloyType(prop),
			Description:  prop.Description,
			DefaultValue: determineDefault(prop),
			Required:     printRequired(isPropRequired),
		})
	}

	tables = append(tables, curPropTable)
	return tables
}

func isBlock(prop *metadata.Schema) bool {
	if prop.Type != "object" {
		return false
	}

	// The property is an object, but the schema explicitly said it's not a block
	// TODO: Make "block" an enum
	if prop.Alloy.Type != "" && prop.Alloy.Type != "block" {
		return false
	}

	return true
}

func printRequired(required bool) string {
	if required {
		return "yes"
	}
	return "no"
}

func toAlloyType(prop *metadata.Schema) string {
	if prop.Alloy.TypeOverride != "" {
		return prop.Alloy.TypeOverride
	}

	switch prop.Type {
	case "integer":
		return "int"
	case "boolean":
		return "bool"
	default:
		return prop.Type
	}
}

func sortArgTables(tables []ArgTable) {
	for _, table := range tables {
		slices.SortStableFunc(table.Rows, func(i, j ArgTableRow) int {
			// TODO: Sort by required arguments first
			return strings.Compare(i.Name, j.Name)
		})
	}
}

func sortBlocksTables(table BlocksTable) {
	slices.SortStableFunc(table.Rows, func(i, j BlocksTableRow) int {
		// TODO: Sort by required blocks first
		return strings.Compare(i.HierarchyChain, j.HierarchyChain)
	})
}

func sortExportsTables(table ExportsTable) {
	slices.SortStableFunc(table.Rows, func(i, j ExportsTableRow) int {
		return strings.Compare(i.Name, j.Name)
	})
}

// generateReferenceTable generates a markdown table from the schema
func generateMarkdownTables(arguments []ArgTable, blocks BlocksTable, exports ExportsTable) (map[string]string, error) {
	res := make(map[string]string)
	for _, table := range arguments {
		tableStr, err := generateArgumentsTableMd(table)
		if err != nil {
			return nil, err
		}
		res[table.Name] = tableStr
	}

	// TODO: Generate blocks link refs
	blocksTableStr, err := generateBlocksTableMd(blocks)
	if err != nil {
		return nil, err
	}
	res["__blocks"] = blocksTableStr

	exportsTableStr, err := generateExportsTableMd(exports)
	if err != nil {
		return nil, err
	}
	res["__exports"] = exportsTableStr

	return res, nil
}

func generateArgumentsTableMd(table ArgTable) (string, error) {
	if len(table.Rows) == 0 {
		return "There are no arguments.", nil
	}

	var sb strings.Builder

	// Write table header
	sb.WriteString("| Name  | Type  | Description  | Default  | Required |\n")
	sb.WriteString("| ----- | ----- | ------------ | -------- | -------- |\n")

	// Process only argument properties (filter by alloy.io == "argument")
	for _, prop := range table.Rows {
		// Write table row
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s | %s |\n",
			// TODO: Fix the "required" later
			prop.Name, prop.Type, prop.Description, markdownCode(prop.DefaultValue), prop.Required))
	}

	return sb.String(), nil
}

func markdownCode(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("`%s`", s)
}

func generateExportsTableMd(table ExportsTable) (string, error) {
	if len(table.Rows) == 0 {
		return "There are no exports.", nil
	}

	var sb strings.Builder

	sb.WriteString("| Name | Type | Description |\n")
	sb.WriteString("| ---- | ---- | ----------- |\n")

	for _, prop := range table.Rows {
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
			prop.Name, prop.Type, prop.Description))
	}

	return sb.String(), nil
}

func generateBlocksTableMd(table BlocksTable) (string, error) {
	if len(table.Rows) == 0 {
		return "There are no blocks.", nil
	}

	var sb strings.Builder

	sb.WriteString("| Block | Description | Required |\n")
	sb.WriteString("| ----- | ----------- | -------- |\n")

	for _, prop := range table.Rows {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			prop.HierarchyChain, prop.Description, prop.Required))
	}

	sb.WriteString("\n")

	for _, prop := range table.Rows {
		// TODO: What if there are two blocks with the same name?
		// TODO: Search and replace various characters? I think markdown doesn't handle everything.
		sb.WriteString(fmt.Sprintf("[%s]: #%s\n", prop.Name, prop.Name))
	}

	return sb.String(), nil
}

// determineDefault determines the default value display for a property
func determineDefault(prop *metadata.Schema) string {
	if prop.Alloy.DefaultOverride != "" {
		return prop.Alloy.DefaultOverride
	}

	if prop.Type == "string" {
		if str, ok := prop.Default.(string); ok {
			return fmt.Sprintf("%q", str)
		}
	}

	if prop.Default != nil {
		return fmt.Sprintf("%v", prop.Default)
	}
	return "" // Empty default
}
