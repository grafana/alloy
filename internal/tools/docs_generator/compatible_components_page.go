package docs_generator

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/grafana/alloy/internal/component/metadata"
)

type CompatibleComponentsListGenerator struct {
	filePath    string
	t           metadata.Type
	sectionName string
	generateFn  func() string
}

func NewExportersListGenerator(t metadata.Type, filePath string) *CompatibleComponentsListGenerator {
	return &CompatibleComponentsListGenerator{
		filePath:    filePath,
		t:           t,
		sectionName: "exporters",
		generateFn:  func() string { return listOfComponentsExporting(t) },
	}
}

func NewConsumersListGenerator(t metadata.Type, filePath string) *CompatibleComponentsListGenerator {
	return &CompatibleComponentsListGenerator{
		filePath:    filePath,
		t:           t,
		sectionName: "consumers",
		generateFn:  func() string { return listOfComponentsAccepting(t) },
	}
}

func (c *CompatibleComponentsListGenerator) Name() string {
	return fmt.Sprintf("generator of %s section for %q in %q", c.sectionName, c.t.Name, c.filePath)
}

func (c *CompatibleComponentsListGenerator) Generate() (string, error) {
	return c.generateFn(), nil
}

func (c *CompatibleComponentsListGenerator) Read() (string, error) {
	content, err := readBetweenMarkers(c.startMarker(), c.endMarker(), c.filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read existing content for %q: %w", c.Name(), err)
	}
	return content, err
}

func (c *CompatibleComponentsListGenerator) Write() error {
	newSection, err := c.Generate()
	if err != nil {
		return err
	}
	if strings.TrimSpace(newSection) == "" {
		return nil
	}
	newSection = "\n" + newSection + "\n"
	return writeBetweenMarkers(c.startMarker(), c.endMarker(), c.filePath, newSection, false)
}

func (c *CompatibleComponentsListGenerator) startMarker() string {
	return fmt.Sprintf("<!-- START GENERATED SECTION: %s OF %s -->", strings.ToUpper(c.sectionName), c.t.Name)
}

func (c *CompatibleComponentsListGenerator) endMarker() string {
	return fmt.Sprintf("<!-- END GENERATED SECTION: %s OF %s -->", strings.ToUpper(c.sectionName), c.t.Name)
}

func listOfComponentsAccepting(dataType metadata.Type) string {
	return listOfLinksToComponents(allComponentsThatAccept(dataType))
}

func listOfComponentsExporting(dataType metadata.Type) string {
	return listOfLinksToComponents(allComponentsThatExport(dataType))
}

func listOfLinksToComponents(components []string) string {
	str := ""
	groups := make(map[string][]string)

	for _, component := range components {
		parts := strings.SplitN(component, ".", 2)
		namespace := parts[0]
		groups[namespace] = append(groups[namespace], component)
	}

	sortedNamespaces := maps.Keys(groups)
	sort.Strings(sortedNamespaces)

	for _, namespace := range sortedNamespaces {
		str += fmt.Sprintf("\n{{< collapse title=%q >}}\n", namespace)
		for _, component := range groups[namespace] {
			str += fmt.Sprintf("- [%[2]s](../components/%[1]s/%[2]s)\n", namespace, component)
		}
		str += "{{< /collapse >}}\n"
	}
	return str
}
