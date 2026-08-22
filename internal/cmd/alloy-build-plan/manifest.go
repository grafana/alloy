package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"unicode"

	"github.com/grafana/alloy/internal/component/all/catalog"
	"gopkg.in/yaml.v3"
)

const (
	buildTagsEnv  = "dist.build_tags"
	yamlStringTag = "!!str"
)

type delegateInvocation struct {
	Name   string
	Args   []string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type delegate interface {
	Run(delegateInvocation) (int, error)
}

type execDelegate struct{}

func (execDelegate) Run(invocation delegateInvocation) (int, error) {
	// #nosec G204 -- this internal build planner supplies the command and arguments.
	cmd := exec.Command(invocation.Name, invocation.Args...)
	cmd.Env = invocation.Env
	cmd.Stdin = invocation.Stdin
	cmd.Stdout = invocation.Stdout
	cmd.Stderr = invocation.Stderr
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

func decodeYAMLDocument(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	return &document, nil
}

func encodeYAMLDocument(document *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func documentMapping(document *yaml.Node) *yaml.Node {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil
	}
	if document.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return document.Content[0]
}

func validateMappingKeys(node *yaml.Node, visited map[*yaml.Node]struct{}) error {
	if node == nil {
		return nil
	}
	if _, exists := visited[node]; exists {
		return nil
	}
	visited[node] = struct{}{}

	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are not supported in Alloy builder manifests (alias at line %d)", node.Line)
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("mapping at line %d has an unmatched key", node.Line)
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != yamlStringTag {
				return fmt.Errorf("mapping key at line %d must be a string", key.Line)
			}
			if _, exists := seen[key.Value]; exists {
				return fmt.Errorf("duplicate mapping key %q at line %d", key.Value, key.Line)
			}
			seen[key.Value] = struct{}{}
			if err := validateMappingKeys(node.Content[i+1], visited); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := validateMappingKeys(child, visited); err != nil {
			return err
		}
	}
	return nil
}

func mappingValue(mapping *yaml.Node, name string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind == yaml.ScalarNode && key.Tag == yamlStringTag && key.Value == name {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

func removeMappingKey(mapping *yaml.Node, name string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind == yaml.ScalarNode && key.Tag == yamlStringTag && key.Value == name {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

func parseAlloyComponents(node *yaml.Node) ([]string, error) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, errors.New("must be a YAML list (use [] to select no native components)")
	}
	components := make([]string, 0, len(node.Content))
	for index, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != yamlStringTag || item.Value == "" {
			return nil, fmt.Errorf("item %d must be a non-empty string", index)
		}
		components = append(components, item.Value)
	}
	return components, nil
}

func manifestBuildTags(root *yaml.Node) (string, error) {
	distribution, found := mappingValue(root, "dist")
	if !found {
		return "", nil
	}
	if distribution.Kind != yaml.MappingNode {
		return "", errors.New("dist must be a mapping")
	}
	buildTags, found := mappingValue(distribution, "build_tags")
	if !found {
		return "", nil
	}
	if buildTags.Kind != yaml.ScalarNode || buildTags.Tag != yamlStringTag {
		return "", errors.New("dist.build_tags must be a string")
	}
	return buildTags.Value, nil
}

func setManifestBuildTags(root *yaml.Node, value string) {
	distribution, found := mappingValue(root, "dist")
	if !found {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "dist"},
			&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
		)
		distribution = root.Content[len(root.Content)-1]
	}
	buildTags, found := mappingValue(distribution, "build_tags")
	if !found {
		distribution.Content = append(distribution.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "build_tags"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag},
		)
		buildTags = distribution.Content[len(distribution.Content)-1]
	}
	buildTags.Kind = yaml.ScalarNode
	buildTags.Tag = yamlStringTag
	buildTags.Style = 0
	buildTags.Value = value
}

func validateBuildTagList(tags []string, source string) ([]string, error) {
	unique := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if tag == catalog.CustomBuildTag || strings.HasPrefix(tag, "alloy_component_") {
			return nil, fmt.Errorf("%s contains reserved Alloy build tag %q; configure native components with alloy_components instead", source, tag)
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		unique = append(unique, tag)
	}
	return unique, nil
}

func splitBuildTags(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

func removeBuildTagsEnvironment(environ []string) ([]string, string, bool) {
	prefix := buildTagsEnv + "="
	filtered := make([]string, 0, len(environ))
	value := ""
	found := false
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			value = strings.TrimPrefix(entry, prefix)
			found = true
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, value, found
}

func reportf(writer io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(writer, format, args...)
}
