// Command otelgen generates the wrappers listed in the specs packages. The only
// argument is the path to the otelcol package directory.
//
// Generated tests are snapshots of upstream behaviour and are only written when
// missing, unless -update-tests is set.
package main

//go:generate go run . ../..

import (
	"flag"
	"log"

	"github.com/grafana/alloy/internal/component/otelcol/otelgen/specs"
)

func main() {
	updateTests := flag.Bool("update-tests", false, "overwrite existing generated tests")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("usage: otelgen [-update-tests] <otelcol dir>")
	}
	root := flag.Arg(0)

	for _, c := range specs.All {
		if err := generate(c, root, *updateTests); err != nil {
			log.Fatalf("%s: %v", fullName(c), err)
		}
	}
}
