// Command rivereval reads a River file from disk, evaluates it as an
// expression, and prints the result as a River value.
package main

import (
	"fmt"
	"os"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/token/builder"
	"github.com/grafana/river/vm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	args := os.Args[1:]

	if len(args) != 1 {
		return fmt.Errorf("usage: rivereval [file]")
	}

	contents, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	// We currently can't support parsing entire files since eval.Evaluate
	// assumes you'll pass a block with a struct schema to it. This might be a
	// restriction we can loosen in the future.
	node, err := parser.ParseExpression(string(contents))
	if err != nil {
		return err
	}
	eval := vm.New(node)

	var v interface{}
	if err := eval.Evaluate(nil, &v); err != nil {
		return err
	}

	expr := builder.NewExpr()
	expr.SetValue(v)

	_, _ = expr.WriteTo(os.Stdout)
	fmt.Println() // Write an extra newline at the end
	return nil
}
