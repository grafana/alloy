// Command beyla-config-validator validates a generated Beyla config against the
// upstream beyla.Config struct. See README.md.
package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/grafana/beyla/v3/pkg/beyla"
	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: beyla-config-validator <config.yaml>")
		os.Exit(2)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg beyla.Config
	if err := dec.Decode(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s does not match beyla.Config:\n%v\n", os.Args[1], err)
		os.Exit(1)
	}

	fmt.Printf("%s: every key and value maps to beyla.Config\n", os.Args[1])
}
