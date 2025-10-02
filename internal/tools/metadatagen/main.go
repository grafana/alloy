package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"unicode"

	"github.com/grafana/alloy/internal/tools/metadatagen/internal"
	"github.com/grafana/alloy/internal/tools/metadatagen/internal/jsonschema"
)

const metadataFileName = "metadata.yml"

//go:embed templates/struct.tmpl
var structTmpl []byte

//go:embed templates/file.tmpl
var fileTmpl []byte

var stmpl = template.Must(template.New("tmpl").Parse(string(structTmpl)))
var ftmpl = template.Must(template.New("tmpl").Parse(string(fileTmpl)))

func main() {
	if len(os.Args) < 1 {
		fmt.Println("Missing required path")
		fmt.Fprint(os.Stderr, "Missing required path\n")
		os.Exit(1)
	}

	out, err := genFromMetadata(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate code: %s\n", err)
		os.Exit(1)
	}

	fmt.Print(out)
}

type FileArgs struct {
	Package string
	Imports []string
	Structs []string
}

func genFromMetadata(path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(path, metadataFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read metadata file: %w", err)
	}

	metadata, err := jsonschema.ParseMetadata(data)
	if err != nil {
		return "", fmt.Errorf("failed to parse metadata file: %w", err)
	}

	fileArgs := FileArgs{}

	if err := genAlloyStruct(&fileArgs, path, "Exports", metadata.Exports); err != nil {
		return "", fmt.Errorf("failed to generate go struct from exports: %w", err)
	}

	if err := genAlloyStruct(&fileArgs, path, "Arguments", metadata.Arguments); err != nil {
		return "", fmt.Errorf("failed to generate go struct from arguments: %w", err)
	}

	pkg, err := internal.PackageFromPath(path)
	if err != nil {
		return "", fmt.Errorf("faild to determine go package: %w", err)
	}
	fileArgs.Package = pkg

	b := strings.Builder{}
	if err := ftmpl.Execute(&b, fileArgs); err != nil {
		return "", fmt.Errorf("failed to generate file: %w", err)
	}

	return b.String(), nil
}

type StructArgs struct {
	Package string
	Name    string
	Fields  []StructField
}

type StructField struct {
	Description *string
	Name        string
	Type        string
	Tag         string
}

func genAlloyStruct(fileArgs *FileArgs, path string, name string, s *jsonschema.Schema) error {
	if !s.IsObject() {
		return fmt.Errorf("expected type object got %s", s.Type)
	}

	pkg, err := internal.PackageFromPath(path)
	if err != nil {
		return fmt.Errorf("faild to determine go package: %w", err)
	}

	args := StructArgs{
		Package: pkg,
		Name:    name,
		Fields:  []StructField{},
	}

	required := make(map[string]struct{}, len(s.Required))
	for _, r := range s.Required {
		required[r] = struct{}{}
	}

	for name, p := range *s.Properties {
		tp := StructField{
			Description: p.Description,
			Name:        formatName(name),
		}

		tp.Type = p.GoType()
		if p.Alloy.TypeSource != "" && !slices.Contains(fileArgs.Imports, p.Alloy.TypeSource) {
			fileArgs.Imports = append(fileArgs.Imports, p.Alloy.TypeSource)
		}

		tag := strings.Builder{}
		tag.WriteString("`alloy:\"")
		tag.WriteString(name)

		// FIXME: block, attributes and enum
		tag.WriteString(",attr")

		if _, ok := required[name]; !ok {
			tag.WriteString(",optional")
		}

		tag.WriteString("\"`")
		tp.Tag = tag.String()
		args.Fields = append(args.Fields, tp)
	}

	b := strings.Builder{}
	if err := stmpl.Execute(&b, args); err != nil {
		return fmt.Errorf("failed to build %s: %w", name, err)
	}
	fileArgs.Structs = append(fileArgs.Structs, b.String())
	return nil
}

// formatName will make first letter uppercase, it will also remove any underscores and make following letter uppercase.
func formatName(name string) string {
	b := strings.Builder{}
	nextUpper := true

	for _, r := range []rune(name) {
		if nextUpper {
			nextUpper = false
			b.WriteRune(unicode.ToUpper(r))
			continue
		}

		if r == '_' {
			nextUpper = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
