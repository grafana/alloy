// Command generate maintains the build-tagged component imports in the parent package.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	catalogVersion        = 1
	componentImportPrefix = "github.com/grafana/alloy/internal/component/"
	customBuildTag        = "alloy_custom_components"
	customFilePrefix      = "custom_"
	fullImportFile        = "all.go"
	makeCatalogFile       = "catalog.mk"
)

var componentNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

type catalog struct {
	Version  int              `json:"version"`
	Packages []catalogPackage `json:"packages"`
}

type catalogPackage struct {
	ImportPath string   `json:"package"`
	Components []string `json:"components"`
}

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
		catalogPath := fs.String("catalog", "catalog.json", "path to the component catalog")
		repoRoot := fs.String("repo-root", "../../..", "path to the Alloy repository root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("audit does not accept positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		cat, err := loadCatalog(*catalogPath)
		if err != nil {
			return err
		}
		return auditCatalogSources(cat, *repoRoot)

	case "generate":
		fs := flag.NewFlagSet("generate", flag.ContinueOnError)
		fs.SetOutput(stderr)
		catalogPath := fs.String("catalog", "catalog.json", "path to the component catalog")
		outDir := fs.String("out", ".", "directory containing package all")
		check := fs.Bool("check", false, "verify generated files without changing them")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("generate does not accept positional arguments: %s", strings.Join(fs.Args(), " "))
		}
		cat, err := loadCatalog(*catalogPath)
		if err != nil {
			return err
		}
		files, err := renderCatalog(cat)
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
		catalogPath := fs.String("catalog", "catalog.json", "path to the component catalog")
		components := fs.String("components", "", "comma- or space-separated Alloy component names; use all for every component or none for no components")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		cat, err := loadCatalog(*catalogPath)
		if err != nil {
			return err
		}
		selection := selectionArgs(*components, fs.Args())
		tags, err := tagsForSelection(cat, selection)
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
  generate audit [-catalog catalog.json] [-repo-root ../../..]
  generate generate [-catalog catalog.json] [-out .] [-check]
  generate validate [-catalog catalog.json] [-components "name,..."] [name ...]
  generate tags [-catalog catalog.json] [-components "name,..."] [name ...]
`)
	return err
}

func loadCatalog(filename string) (catalog, error) {
	f, err := os.Open(filename)
	if err != nil {
		return catalog{}, fmt.Errorf("open catalog: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var cat catalog
	if err := dec.Decode(&cat); err != nil {
		return catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return catalog{}, err
	}
	if err := validateCatalog(cat); err != nil {
		return catalog{}, fmt.Errorf("validate catalog: %w", err)
	}
	return cat, nil
}

func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode catalog: multiple JSON values")
		}
		return fmt.Errorf("decode catalog trailing data: %w", err)
	}
	return nil
}

func validateCatalog(cat catalog) error {
	if cat.Version != catalogVersion {
		return fmt.Errorf("unsupported version %d (want %d)", cat.Version, catalogVersion)
	}
	if len(cat.Packages) == 0 {
		return errors.New("catalog has no packages")
	}

	seenPackages := make(map[string]struct{}, len(cat.Packages))
	seenComponents := make(map[string]string)
	seenTags := make(map[string]string)
	seenFiles := make(map[string]string)
	previousPackage := ""
	for packageIndex, pkg := range cat.Packages {
		if !strings.HasPrefix(pkg.ImportPath, componentImportPrefix) {
			return fmt.Errorf("package %q is outside %q", pkg.ImportPath, componentImportPrefix)
		}
		if path.Clean(pkg.ImportPath) != pkg.ImportPath {
			return fmt.Errorf("package %q is not a clean import path", pkg.ImportPath)
		}
		if packageIndex > 0 && pkg.ImportPath <= previousPackage {
			return fmt.Errorf("packages are not strictly sorted: %q follows %q", pkg.ImportPath, previousPackage)
		}
		previousPackage = pkg.ImportPath
		if _, exists := seenPackages[pkg.ImportPath]; exists {
			return fmt.Errorf("duplicate package %q", pkg.ImportPath)
		}
		seenPackages[pkg.ImportPath] = struct{}{}
		if len(pkg.Components) == 0 {
			return fmt.Errorf("package %q has no components", pkg.ImportPath)
		}

		filename := customFilename(pkg.ImportPath)
		if other, exists := seenFiles[filename]; exists {
			return fmt.Errorf("generated filename %q collides for %q and %q", filename, other, pkg.ImportPath)
		}
		seenFiles[filename] = pkg.ImportPath

		previousComponent := ""
		for componentIndex, name := range pkg.Components {
			if !componentNamePattern.MatchString(name) {
				return fmt.Errorf("package %q has invalid component name %q", pkg.ImportPath, name)
			}
			if componentIndex > 0 && name <= previousComponent {
				return fmt.Errorf("components for package %q are not strictly sorted: %q follows %q", pkg.ImportPath, name, previousComponent)
			}
			previousComponent = name
			if other, exists := seenComponents[name]; exists {
				return fmt.Errorf("component %q is registered by both %q and %q", name, other, pkg.ImportPath)
			}
			seenComponents[name] = pkg.ImportPath

			tag := componentBuildTag(name)
			if other, exists := seenTags[tag]; exists {
				return fmt.Errorf("build tag %q collides for components %q and %q", tag, other, name)
			}
			seenTags[tag] = name
		}
	}
	return nil
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

func tagsForSelection(cat catalog, selection []string) ([]string, error) {
	known := make(map[string]struct{})
	for _, pkg := range cat.Packages {
		for _, name := range pkg.Components {
			known[name] = struct{}{}
		}
	}

	switch {
	case len(selection) == 1 && selection[0] == "all":
		selection = selection[:0]
		for name := range known {
			selection = append(selection, name)
		}
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

	seen := make(map[string]struct{}, len(selection))
	for _, name := range selection {
		if _, exists := known[name]; !exists {
			return nil, fmt.Errorf("unknown Alloy component %q", name)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate Alloy component %q", name)
		}
		seen[name] = struct{}{}
	}
	sort.Strings(selection)

	tags := make([]string, 1, len(selection)+1)
	tags[0] = customBuildTag
	for _, name := range selection {
		tags = append(tags, componentBuildTag(name))
	}
	return tags, nil
}

func componentBuildTag(name string) string {
	return "alloy_component_" + normalizeComponentName(name)
}

func normalizeComponentName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func customFilename(importPath string) string {
	relative := strings.TrimPrefix(importPath, componentImportPrefix)
	return customFilePrefix + strings.ReplaceAll(relative, "/", "_") + ".go"
}

func renderCatalog(cat catalog) (map[string][]byte, error) {
	if err := validateCatalog(cat); err != nil {
		return nil, err
	}
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
