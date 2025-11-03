package alloyyaml

import (
	"fmt"
	"testing"
	"gopkg.in/yaml.v3"
)

func TestYAMLTags(t *testing.T) {
	yamlData := `
this_is_a_block:
  label_name:
    key1: value1
    key2: value2

this_is_an_object: !!map
  key1: value1
  key2: value2
`
	
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(yamlData), &node); err != nil {
		t.Fatal(err)
	}
	
	printNode(&node, 0)
}

func printNode(node *yaml.Node, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	
	kindStr := "unknown"
	switch node.Kind {
	case yaml.DocumentNode:
		kindStr = "Document"
	case yaml.MappingNode:
		kindStr = "Mapping"
	case yaml.SequenceNode:
		kindStr = "Sequence"
	case yaml.ScalarNode:
		kindStr = "Scalar"
	case yaml.AliasNode:
		kindStr = "Alias"
	}
	
	fmt.Printf("%sKind: %s, Tag: %s, Value: %q\n", indent, kindStr, node.Tag, node.Value)
	
	for _, child := range node.Content {
		printNode(child, depth+1)
	}
}
