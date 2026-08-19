// Command generate maintains the build-tagged component imports in the parent package.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	componentcatalog "github.com/grafana/alloy/internal/component/all/catalog"
)

const (
	componentImportPrefix = "github.com/grafana/alloy/internal/component/"
	customBuildTag        = componentcatalog.CustomBuildTag
	customFilePrefix      = "custom_"
	fullImportFile        = "all.go"
	makeCatalogFile       = "catalog.mk"
)

type catalog struct {
	Packages []componentcatalog.Entry
}

type catalogPackage = componentcatalog.Entry

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "component import generator:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		if err := printUsage(stderr); err != nil {
			return fmt.Errorf("print usage: %w", err)
		}
		return errors.New("missing command")
	}

	switch args[0] {
	case "audit":
		fs := flag.NewFlagSet("audit", flag.ContinueOnError)
		fs.SetOutput(stderr)
		repoRoot := fs.String("repo-root", "../../..", "path to the Alloy repository root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("audit does not accept positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		return auditCatalogSources(loadCatalog(), *repoRoot)

	case "generate":
		fs := flag.NewFlagSet("generate", flag.ContinueOnError)
		fs.SetOutput(stderr)
		outDir := fs.String("out", ".", "directory containing package all")
		check := fs.Bool("check", false, "verify generated files without changing them")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("generate does not accept positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		files, err := renderCatalog(loadCatalog())
		if err != nil {
			return err
		}
		if *check {
			return checkGenerated(*outDir, files)
		}
		return writeGenerated(*outDir, files)

	case "validate", "tags":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		fs.SetOutput(stderr)
		components := fs.String("components", "", "comma- or space-separated Alloy component names; use all for every component or none for no components")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		selection := selectionArgs(*components, fs.Args())
		tags, err := tagsForSelection(selection)
		if err != nil {
			return err
		}
		if args[0] == "tags" {
			_, err = fmt.Fprintln(stdout, strings.Join(tags, ","))
			return err
		}
		return nil

	default:
		if err := printUsage(stderr); err != nil {
			return fmt.Errorf("print usage: %w", err)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `usage:
  generate audit [-repo-root ../../..]
  generate generate [-out .] [-check]
  generate validate [-components "name,..."] [name ...]
  generate tags [-components "name,..."] [name ...]
`)
	return err
}

func loadCatalog() catalog {
	return catalog{Packages: componentcatalog.Entries()}
}

func selectionArgs(flagValue string, args []string) []string {
	selection := splitSelection(flagValue)
	for _, arg := range args {
		selection = append(selection, splitSelection(arg)...)
	}
	return selection
}

func splitSelection(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

func tagsForSelection(selection []string) ([]string, error) {
	switch {
	case len(selection) == 1 && selection[0] == "all":
		selection = componentcatalog.Names()
	case len(selection) == 1 && selection[0] == "none":
		selection = selection[:0]
	default:
		for _, name := range selection {
			switch name {
			case "all":
				return nil, errors.New(`component selector "all" cannot be combined with other selectors`)
			case "none":
				return nil, errors.New(`component selector "none" cannot be combined with other selectors`)
			}
		}
	}

	componentTags, err := componentcatalog.ResolveExact(selection)
	if err != nil {
		return nil, err
	}
	tags := make([]string, 1, len(componentTags)+1)
	tags[0] = customBuildTag
	tags = append(tags, componentTags...)
	return tags, nil
}

func componentBuildTag(name string) string {
	tag, ok := componentcatalog.TagForName(name)
	if !ok {
		panic(fmt.Sprintf("component %q is not in the catalog", name))
	}
	return tag
}

func normalizeComponentName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func customFilename(importPath string) string {
	relative := strings.TrimPrefix(importPath, componentImportPrefix)
	return customFilePrefix + strings.ReplaceAll(relative, "/", "_") + ".go"
}

func renderCatalog(cat catalog) (map[string][]byte, error) {
	files := make(map[string][]byte, len(cat.Packages)+2)

	full, err := renderFullImportFile(cat)
	if err != nil {
		return nil, err
	}
	files[fullImportFile] = full
	for _, pkg := range cat.Packages {
		content, err := renderCustomImportFile(pkg)
		if err != nil {
			return nil, err
		}
		files[customFilename(pkg.ImportPath)] = content
	}
	files[makeCatalogFile] = renderMakeCatalog(cat)
	return files, nil
}

func renderMakeCatalog(cat catalog) []byte {
	var names []string
	for _, pkg := range cat.Packages {
		names = append(names, pkg.Components...)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("# Code generated by internal/component/all/generate. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "ALLOY_CUSTOM_COMPONENTS_TAG := %s\n\n", customBuildTag)
	out.WriteString("ALLOY_COMPONENT_NAMES := \\\n")
	for index, name := range names {
		continuation := " \\\n"
		if index == len(names)-1 {
			continuation = "\n"
		}
		fmt.Fprintf(&out, "\t%s%s", name, continuation)
	}
	out.WriteString("\n")
	for _, name := range names {
		fmt.Fprintf(&out, "ALLOY_COMPONENT_TAG_%s := %s\n", normalizeComponentName(name), componentBuildTag(name))
	}
	return []byte(out.String())
}

func renderFullImportFile(cat catalog) ([]byte, error) {
	var src strings.Builder
	src.WriteString("//go:build !alloy_custom_components\n\n")
	src.WriteString("// Code generated by internal/component/all/generate. DO NOT EDIT.\n\n")
	src.WriteString("package all\n\nimport (\n")
	for _, pkg := range cat.Packages {
		fmt.Fprintf(&src, "\t_ %q // %s\n", pkg.ImportPath, importComment(pkg.Components))
	}
	src.WriteString(")\n")
	return formatGenerated(fullImportFile, src.String())
}

func renderCustomImportFile(pkg catalogPackage) ([]byte, error) {
	tags := make([]string, 0, len(pkg.Components))
	for _, name := range pkg.Components {
		tags = append(tags, componentBuildTag(name))
	}
	componentExpression := strings.Join(tags, " || ")
	if len(tags) > 1 {
		componentExpression = "(" + componentExpression + ")"
	}

	var src strings.Builder
	fmt.Fprintf(&src, "//go:build %s && %s\n\n", customBuildTag, componentExpression)
	src.WriteString("// Code generated by internal/component/all/generate. DO NOT EDIT.\n\n")
	src.WriteString("package all\n\n")
	fmt.Fprintf(&src, "import _ %q // %s\n", pkg.ImportPath, importComment(pkg.Components))
	return formatGenerated(customFilename(pkg.ImportPath), src.String())
}

func importComment(names []string) string {
	switch len(names) {
	case 0:
		panic("importComment called without names")
	case 1:
		return "Import " + names[0]
	case 2:
		return "Import " + names[0] + " and " + names[1]
	default:
		return "Import " + strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func formatGenerated(filename, source string) ([]byte, error) {
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return nil, fmt.Errorf("format %s: %w", filename, err)
	}
	return formatted, nil
}

func writeGenerated(outDir string, files map[string][]byte) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	for _, filename := range sortedFilenames(files) {
		if err := os.WriteFile(filepath.Join(outDir, filename), files[filename], 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}
	return removeStaleGeneratedFiles(outDir, files, false)
}

func checkGenerated(outDir string, files map[string][]byte) error {
	var problems []string
	for _, filename := range sortedFilenames(files) {
		actual, err := os.ReadFile(filepath.Join(outDir, filename))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", filename, err))
			continue
		}
		if !bytes.Equal(actual, files[filename]) {
			problems = append(problems, filename+": content is stale")
		}
	}
	if err := removeStaleGeneratedFiles(outDir, files, true); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func removeStaleGeneratedFiles(outDir string, expected map[string][]byte, checkOnly bool) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("read output directory: %w", err)
	}
	var stale []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, customFilePrefix) || !strings.HasSuffix(name, ".go") {
			continue
		}
		if _, exists := expected[name]; exists {
			continue
		}
		if checkOnly {
			stale = append(stale, name)
			continue
		}
		if err := os.Remove(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("remove stale generated file %s: %w", name, err)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		return fmt.Errorf("stale generated files: %s", strings.Join(stale, ", "))
	}
	return nil
}

func sortedFilenames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
