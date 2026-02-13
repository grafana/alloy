package validate

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"slices"

	"github.com/grafana/alloy/internal/tools/alloygen/metadata"
	"github.com/grafana/alloy/internal/tools/alloygen/validate/internal"
)

const metadataFileName = "metadata.yml"

// TODO: A smarter way to ignore paths? Or simply to know what to validate?
var ignoreList []string

func init() {
	ignoreList = []string{
		"./internal/tools/alloygen/testdata/test1",
	}
}

func Run(args []string) error {
	if len(args) < 1 {
		return errors.New("Missing required path")
	}

	if slices.Contains(ignoreList, args[0]) {
		log.Printf("Ignoring path: %s", args[0])
		return nil
	}

	log.Printf("Processing path: %s", args[0])

	api, err := internal.Parse(args[0])
	if err != nil {
		return fmt.Errorf("failed to parse api: %w", err)
	}

	metadata, err := metadata.FromPath(filepath.Join(args[0], metadataFileName))
	if err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	if err := validate(api, metadata); err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}

	return nil
}

func validate(api *internal.API, metadata *metadata.Metadata) error {
	if err := validateStruct("Arguments", api.Arguments, metadata.Arguments); err != nil {
		return fmt.Errorf("invalid arguments: %s", err)
	}

	if err := validateStruct("Exports", api.Exports, metadata.Exports); err != nil {
		return fmt.Errorf("invalid exports: %s", err)
	}
	return nil
}

func validateStruct(typ string, s *internal.Struct, schema *metadata.Schema) error {
	if s == nil && schema == nil {
		return nil
	}

	if s == nil && schema != nil {
		return fmt.Errorf("%s is defined in schema but not in code", typ)
	}

	if s != nil && schema == nil {
		return fmt.Errorf("%s is defined in code but not in schema", typ)
	}

	for _, f := range s.Fields {
		// FIXME: handle squashed properties
		if len(f.Tag) == 1 && f.Tag[0] == "squash" {
			continue
		}

		if len(f.Tag) < 1 {
			return fmt.Errorf("malformat tag for field %s", f.Name)
		}

		if _, ok := schema.Properties[f.Tag[0]]; !ok {
			return fmt.Errorf("property present in code but missing in schema %s", f.Tag[0])
		}
	}

	for name := range schema.Properties {
		if !slices.ContainsFunc(s.Fields, func(f internal.StructField) bool {
			return name == f.Tag[0]
		}) {
			return fmt.Errorf("property is present in schema but missing in code: %s", name)
		}
	}

	return nil
}
