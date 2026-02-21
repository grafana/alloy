package docs_generator

import (
	"bytes"
	"fmt"
	"os"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/metadata"
)

type DocsGenerator interface {
	Name() string
	Generate() (string, error)
	Read() (string, error)
	Write() error
}

func allComponentsThat(f func(meta metadata.Metadata) bool) []string {
	var result []string
	for _, name := range component.AllNames() {
		meta, err := metadata.ForComponent(name)
		if err != nil {
			panic(err) // should never happen
		}

		if f(meta) {
			result = append(result, name)
		}
	}
	return result
}

func allComponentsThatExport(dataType metadata.Type) []string {
	return allComponentsThat(func(meta metadata.Metadata) bool {
		return meta.ExportsType(dataType)
	})
}

func allComponentsThatAccept(dataType metadata.Type) []string {
	return allComponentsThat(func(meta metadata.Metadata) bool {
		return meta.AcceptsType(dataType)
	})
}

func writeBetweenMarkers(startMarker string, endMarker string, filePath string, content string, appendIfMissing bool) error {
	fileContents, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	replacement := append(append([]byte(startMarker), []byte(content)...), []byte(endMarker)...)

	before, _, ok := bytes.Cut(fileContents, []byte(startMarker))
	endIndex := bytes.LastIndex(fileContents, []byte(endMarker))
	var newFileContents []byte
	if !ok || endIndex == -1 {
		if !appendIfMissing {
			return fmt.Errorf("required markers %q and %q do not exist in %q", startMarker, endMarker, filePath)
		}
		// Append the new section to the end of the file
		newFileContents = append(fileContents, replacement...)
	} else {
		// Replace the section with the new content
		newFileContents = append(newFileContents, before...)
		newFileContents = append(newFileContents, replacement...)
		newFileContents = append(newFileContents, fileContents[endIndex+len(endMarker):]...)
	}
	err = os.WriteFile(filePath, newFileContents, 0644)
	return err
}

func readBetweenMarkers(startMarker string, endMarker string, filePath string) (string, error) {
	fileContents, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	startIndex := bytes.Index(fileContents, []byte(startMarker))
	endIndex := bytes.LastIndex(fileContents, []byte(endMarker))
	if startIndex == -1 || endIndex == -1 {
		return "", fmt.Errorf("markers not found: %q or %q", startMarker, endMarker)
	}

	return string(fileContents[startIndex+len(startMarker) : endIndex]), nil
}
