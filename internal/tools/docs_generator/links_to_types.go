package docs_generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/grafana/alloy/internal/component/metadata"
)

type LinksToTypesGenerator struct {
	component string
}

func NewLinksToTypesGenerator(component string) *LinksToTypesGenerator {
	return &LinksToTypesGenerator{component: component}
}

func (l *LinksToTypesGenerator) Name() string {
	return fmt.Sprintf("generator of links to types for %q reference page", l.component)
}

func (l *LinksToTypesGenerator) Generate() (string, error) {
	meta, err := metadata.ForComponent(l.component)
	if err != nil {
		return "", err
	}
	if meta.Empty() {
		return "", nil
	}

	heading := "\n## Compatible components\n\n"
	acceptingSection := acceptingComponentsSection(l.component, meta)
	outputSection := outputComponentsSection(l.component, meta)

	if acceptingSection == "" && outputSection == "" {
		return "", nil
	}

	note := `
{{< admonition type="note" >}}
Connecting some components may not be sensible or components may require further configuration to make the connection work correctly.
Refer to the linked documentation for more details.
{{< /admonition >}}
`

	return heading + acceptingSection + outputSection + note, nil
}

func (l *LinksToTypesGenerator) Read() (string, error) {
	content, err := readBetweenMarkers(l.startMarker(), l.endMarker(), l.pathToComponentMarkdown())
	if err != nil {
		return "", fmt.Errorf("failed to read existing content for %q: %w", l.Name(), err)
	}
	return content, err
}

func (l *LinksToTypesGenerator) Write() error {
	newSection, err := l.Generate()
	if err != nil {
		return err
	}
	if strings.TrimSpace(newSection) == "" {
		return nil
	}
	newSection = "\n" + newSection + "\n"
	return writeBetweenMarkers(l.startMarker(), l.endMarker(), l.pathToComponentMarkdown(), newSection, true)
}

func (l *LinksToTypesGenerator) startMarker() string {
	return "<!-- START GENERATED COMPATIBLE COMPONENTS -->"
}

func (l *LinksToTypesGenerator) endMarker() string {
	return "<!-- END GENERATED COMPATIBLE COMPONENTS -->"
}

func (l *LinksToTypesGenerator) pathToComponentMarkdown() string {
	namespaceGuess := strings.Split(l.component, ".")
	if len(namespaceGuess) == 1 {
		panic(fmt.Sprintf("component %q does not seem to have a conventional namespace", l.component))
	}
	namespace := namespaceGuess[0]
	return fmt.Sprintf("../../../docs/sources/reference/components/%s/%s.md", namespace, l.component)
}

func outputComponentsSection(name string, meta metadata.Metadata) string {
	section := ""
	for _, outputDataType := range meta.AllTypesExported() {
		if list := allComponentsThatAccept(outputDataType); len(list) > 0 {
			section += fmt.Sprintf(
				"- Components that consume [%s](../../../compatibility/%s)\n",
				outputDataType.Name,
				anchorFor(outputDataType.Name, "consumers"),
			)
		}
	}
	if section != "" {
		section = fmt.Sprintf("`%s` has exports that can be consumed by the following components:\n\n", name) + section
	}
	return section
}

func acceptingComponentsSection(componentName string, meta metadata.Metadata) string {
	section := ""
	for _, acceptedDataType := range meta.AllTypesAccepted() {
		if list := allComponentsThatExport(acceptedDataType); len(list) > 0 {
			section += fmt.Sprintf(
				"- Components that export [%s](../../../compatibility/%s)\n",
				acceptedDataType.Name,
				anchorFor(acceptedDataType.Name, "exporters"),
			)
		}
	}
	if section != "" {
		section = fmt.Sprintf("`%s` can accept arguments from the following components:\n\n", componentName) + section + "\n"
	}
	return section
}

func anchorFor(parts ...string) string {
	for i, s := range parts {
		reg := regexp.MustCompile("[^a-z0-9-_]+")
		parts[i] = reg.ReplaceAllString(strings.ReplaceAll(strings.ToLower(s), " ", "-"), "")
	}
	return "#" + strings.Join(parts, "-")
}
