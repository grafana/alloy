package main

import (
	"flag"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/internal/tools/schemavalidator/internal"
)

const schemaName = "schema.yml"

func main() {
	var folder string
	flag.StringVar(&folder, "folder", ".", "")
	flag.Parse()

	// Find folders contaning foldersWithSchema
	var foldersWithSchema []string

	filepath.Walk(folder, func(path string, info fs.FileInfo, err error) error {
		if info.Name() == schemaName {
			foldersWithSchema = append(foldersWithSchema, strings.TrimRight(path, info.Name()))
		}
		return nil
	})

	for _, path := range foldersWithSchema {
		err := validateSchema(path)
		if err != nil {
			panic(err)
		}
	}

}

func validateSchema(path string) error {
	api, err := internal.Parse(path)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", api.Arguments)
	fmt.Printf("%+v\n", api.Exports)
	return nil
}
