// Package catalog exposes the canonical inventory of native Alloy components
// that can be selected for a custom build.
package catalog

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	// CustomBuildTag enables Alloy's component-selective build mode.
	CustomBuildTag = "alloy_custom_components"

	catalogVersion        = 1
	componentImportPrefix = "github.com/grafana/alloy/internal/component/"
)

var componentNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

//go:embed catalog.json
var catalogJSON []byte

// Entry describes one component package and the exact runtime component names
// registered by that package.
type Entry struct {
	ImportPath string
	Components []string
}

type document struct {
	Version  int         `json:"version"`
	Packages []jsonEntry `json:"packages"`
}

type jsonEntry struct {
	ImportPath string   `json:"package"`
	Components []string `json:"components"`
}

var inventory = mustParse(catalogJSON)

// Entries returns the catalog entries in canonical import-path order. The
// returned entries and component slices are copies and may be modified by the
// caller.
func Entries() []Entry {
	entries := make([]Entry, len(inventory.entries))
	for i, entry := range inventory.entries {
		entries[i] = Entry{
			ImportPath: entry.ImportPath,
			Components: append([]string(nil), entry.Components...),
		}
	}
	return entries
}

// Names returns every cataloged component name in lexical order. The returned
// slice is a copy and may be modified by the caller.
func Names() []string {
	return append([]string(nil), inventory.names...)
}

// TagForName returns the component-specific build tag for an exact cataloged
// component name.
func TagForName(name string) (string, bool) {
	tag, ok := inventory.tags[name]
	return tag, ok
}

// ResolveExact validates exact component names and returns their
// component-specific build tags in deterministic lexical order. It does not
// add CustomBuildTag; callers that are constructing a custom build must add
// that marker separately.
func ResolveExact(names []string) ([]string, error) {
	ordered := append([]string(nil), names...)
	sort.Strings(ordered)

	var unknown []string
	var duplicates []string
	for i, name := range ordered {
		if i > 0 && name == ordered[i-1] {
			if len(duplicates) == 0 || duplicates[len(duplicates)-1] != name {
				duplicates = append(duplicates, name)
			}
			continue
		}
		if _, ok := inventory.tags[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return nil, componentListError("unknown Alloy component", unknown)
	}
	if len(duplicates) > 0 {
		return nil, componentListError("duplicate Alloy component", duplicates)
	}

	tags := make([]string, len(ordered))
	for i, name := range ordered {
		tags[i] = inventory.tags[name]
	}
	return tags, nil
}

type parsedInventory struct {
	entries []Entry
	names   []string
	tags    map[string]string
}

func mustParse(data []byte) parsedInventory {
	result, err := parse(data)
	if err != nil {
		panic(fmt.Sprintf("invalid embedded Alloy component catalog: %v", err))
	}
	return result
}

func parse(data []byte) (parsedInventory, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var doc document
	if err := dec.Decode(&doc); err != nil {
		return parsedInventory{}, fmt.Errorf("decode catalog: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return parsedInventory{}, err
	}
	if err := validateDocument(doc); err != nil {
		return parsedInventory{}, fmt.Errorf("validate catalog: %w", err)
	}

	result := parsedInventory{
		entries: make([]Entry, len(doc.Packages)),
		tags:    make(map[string]string),
	}
	for i, pkg := range doc.Packages {
		result.entries[i] = Entry{
			ImportPath: pkg.ImportPath,
			Components: append([]string(nil), pkg.Components...),
		}
		for _, name := range pkg.Components {
			result.names = append(result.names, name)
			result.tags[name] = buildTag(name)
		}
	}
	sort.Strings(result.names)
	return result, nil
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

func validateDocument(doc document) error {
	if doc.Version != catalogVersion {
		return fmt.Errorf("unsupported version %d (want %d)", doc.Version, catalogVersion)
	}
	if len(doc.Packages) == 0 {
		return errors.New("catalog has no packages")
	}

	seenPackages := make(map[string]struct{}, len(doc.Packages))
	seenComponents := make(map[string]string)
	seenTags := make(map[string]string)
	seenFiles := make(map[string]string)
	previousPackage := ""
	for packageIndex, pkg := range doc.Packages {
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

		filename := generatedFilename(pkg.ImportPath)
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

			tag := buildTag(name)
			if other, exists := seenTags[tag]; exists {
				return fmt.Errorf("build tag %q collides for components %q and %q", tag, other, name)
			}
			seenTags[tag] = name
		}
	}
	return nil
}

func buildTag(name string) string {
	return "alloy_component_" + strings.ReplaceAll(name, ".", "_")
}

func generatedFilename(importPath string) string {
	relative := strings.TrimPrefix(importPath, componentImportPrefix)
	return "custom_" + strings.ReplaceAll(relative, "/", "_") + ".go"
}

func componentListError(prefix string, names []string) error {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = fmt.Sprintf("%q", name)
	}
	if len(names) > 1 {
		prefix += "s"
	}
	return fmt.Errorf("%s: %s", prefix, strings.Join(quoted, ", "))
}
