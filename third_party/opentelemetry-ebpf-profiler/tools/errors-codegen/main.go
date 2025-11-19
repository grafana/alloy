// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// errors-codegen generates the code containing the host agent error code enums.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

// CodeGen describes the interface to be implemented by each code-generator.
type CodeGen interface {
	Generate(out io.Writer, errors []JSONError) error
}

var codeGens = map[string]CodeGen{
	"bpf": &BPFCodeGen{},
}

type JSONError struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Obsolete    bool   `json:"obsolete,omitempty"`
}

//go:embed errors.json
var errorsJSON []byte

// BPFCodeGen generates a BPF C header file.
type BPFCodeGen struct{}

//go:embed bpf.h.template
var bpfTemplate string

func toCEnumIdent(name string) string {
	return "ERR_" + strings.ToUpper(name)
}

func (cg *BPFCodeGen) Generate(out io.Writer, errors []JSONError) error {
	tmpl := template.New("bpf-template")

	tmpl.Funcs(map[string]any{
		"enumident": toCEnumIdent,
	})

	var err error
	tmpl, err = tmpl.Parse(bpfTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse BPF C header template: %v", err)
	}

	return tmpl.Execute(out, &struct {
		Errors []JSONError
	}{
		Errors: errors,
	})
}

func checkUnique(errors []JSONError) error {
	names := make(map[string]JSONError, len(errors))
	ids := make(map[uint64]JSONError, len(errors))

	for _, item := range errors {
		if existing, exists := names[item.Name]; exists {
			return fmt.Errorf("duplicate name: %#v and %#v", existing, item)
		}
		if existing, exists := ids[item.ID]; exists {
			return fmt.Errorf("duplicate ID: %#v and %#v", existing, item)
		}

		ids[item.ID] = item
		names[item.Name] = item
	}

	return nil
}

func generate(codeGenName, outputPath string) error {
	var entries []JSONError
	if err := json.Unmarshal(errorsJSON, &entries); err != nil {
		return fmt.Errorf("failed to parse `errors.json`: %v", err)
	}

	if err := checkUnique(entries); err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create bpf.h: %v", err)
	}

	cg, exists := codeGens[codeGenName]
	if !exists {
		return fmt.Errorf("unknown code-generator: %s", codeGenName)
	}

	if err = cg.Generate(file, entries); err != nil {
		return fmt.Errorf("failed to do BPF code-gen: %v", err)
	}

	return nil
}

func main() {
	if len(os.Args) != 3 {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s <codegen> <output path>\n", os.Args[0])
		os.Exit(1)
	}

	if err := generate(os.Args[1], os.Args[2]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
