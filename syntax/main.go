package main

import (
	"fmt"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parse"
	"github.com/grafana/alloy/syntax/print"
)

func main() {
	parse.ParseString("")
	// Assuming you have a valid ast.Node instance
	var node ast.Node
	// Initialize node appropriately here

	str, err := print.PrintNode(node)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Output:", str)
}
