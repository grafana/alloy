package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/grafana/alloy/internal/tools/docs_args_generator/jsonschema"
)

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

// Generate markdown files with tables listing all of the arguments, blocks, and exports.
// Read a YAML schema from ymlPath, merge subschemas, and write markdown files to outputPath.
func generate(ymlPath string, outputPath string) error {
	log.Printf("Processing YAML schema: %s", ymlPath)
	log.Printf("Output directory: %s", outputPath)

	schema, err := jsonschema.LoadMetadata(ymlPath)
	if err != nil {
		return err
	}
	if schema == nil {
		return nil // File doesn't exist, skip
	}

	argumentsTables := newArgumentsTables("__arguments", schema.Arguments)
	blocksTable := newBlocksTable([]string{}, schema.Arguments)
	exportsTable := newExportsTable(schema.Exports)

	for _, t := range argumentsTables {
		t.sort()
	}
	blocksTable.sort()
	exportsTable.sort()

	markdownTables := generateMarkdownTables(argumentsTables, blocksTable, exportsTable)

	err = writeFiles(markdownTables, outputPath)
	if err != nil {
		return fmt.Errorf("failed to write markdown tables: %w", err)
	}

	return nil
}

// generateMarkdownTables generates a markdown table from the schema
func generateMarkdownTables(arguments []*ArgTable, blocks *BlocksTable, exports *ExportsTable) map[string]string {
	res := make(map[string]string)
	for _, table := range arguments {
		res[table.Name] = table.markdown()
	}

	// TODO: Generate blocks link refs
	res["__blocks"] = blocks.markdown()

	res["__exports"] = exports.markdown()

	return res
}

func writeFiles(markdownTables map[string]string, outputPath string) error {
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
