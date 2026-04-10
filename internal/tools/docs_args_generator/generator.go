package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/grafana/alloy/internal/tools/docs_args_generator/jsonschema"
)

// generate reads the YAML schema at ymlPath, merges subschemas, and writes
// markdown documentation files to outputPath.
//
// outputPath is the component-specific output directory
// (e.g. docs/sources/shared/generated/components/loki/source/api).
//
// docsBase, when non-empty, is the root of the shared generated-docs tree
// (e.g. docs/sources/shared/generated). Argument tables that originate from
// an imported subschema are written to docsBase/{schema-relative-path}/
// instead of outputPath. When docsBase is empty all files go to outputPath
// (backward-compatible behaviour used by tests).
func generate(ymlPath string, outputPath string, docsBase string) error {
	log.Printf("Processing YAML schema: %s", ymlPath)
	log.Printf("Output directory: %s", outputPath)

	schema, err := jsonschema.LoadMetadata(ymlPath)
	if err != nil {
		return err
	}
	if schema == nil {
		return nil // File doesn't exist, skip
	}

	argumentsTables := newArgumentsTables("__arguments", schema.Arguments, "")
	blocksTable := newBlocksTable([]string{}, "", schema.Arguments)
	exportsTable := newExportsTable(schema.Exports)

	for _, t := range argumentsTables {
		t.sort()
	}
	blocksTable.sort()
	exportsTable.sort()

	files := buildOutputFiles(argumentsTables, blocksTable, exportsTable, outputPath, docsBase)

	if err := writeFiles(files); err != nil {
		return fmt.Errorf("failed to write markdown tables: %w", err)
	}
	return nil
}

// buildOutputFiles computes the full output file path for every generated
// markdown table. Argument tables from imported subschemas are routed to
// docsBase/{schema-relative-path}/ when docsBase is non-empty; all other
// files land in outputPath.
func buildOutputFiles(args []*ArgTable, blocks *BlocksTable, exports *ExportsTable, outputPath, docsBase string) map[string]string {
	files := make(map[string]string)

	for _, t := range args {
		dir := tableOutputDir(t.SourceSchemaID, outputPath, docsBase)
		files[filepath.Join(dir, t.Name+".md")] = t.markdown()
	}

	files[filepath.Join(outputPath, "__blocks.md")] = blocks.markdown()
	files[filepath.Join(outputPath, "__exports.md")] = exports.markdown()

	return files
}

// tableOutputDir returns the directory into which the argument table for a
// block with the given sourceSchemaID should be written.
func tableOutputDir(sourceSchemaID, componentOutputPath, docsBase string) string {
	if sourceSchemaID == "" || docsBase == "" {
		return componentOutputPath
	}
	return filepath.Join(docsBase, schemaIDToRelPath(sourceSchemaID))
}

func writeFiles(files map[string]string) error {
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err = file.WriteString(content); err != nil {
			return err
		}
	}
	return nil
}
