// Command manifest_gen generates internal/component/manifest.yaml: the list of
// every component registered in the Alloy default engine and its stability
// level. Regenerate with `make generate-component-manifest`.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"

	_ "github.com/grafana/alloy/internal/component/all" // Import all component definitions.

	"gopkg.in/yaml.v3"
)

const header = `# This file is generated. DO NOT EDIT.
# Regenerate with ` + "`make generate-component-manifest`" + `.
#
# Lists every component registered in the Alloy default engine and its
# stability. The Alloy version is the git tag this file is committed under.
`

type entry struct {
	Name      string `yaml:"name"`
	Stability string `yaml:"stability,omitempty"`
	Community bool   `yaml:"community,omitempty"`
}

type manifest struct {
	Components []entry `yaml:"components"`
}

func buildManifest(regs []component.Registration) (manifest, error) {
	sorted := make([]component.Registration, len(regs))
	copy(sorted, regs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	m := manifest{Components: make([]entry, 0, len(sorted))}
	for _, reg := range sorted {
		e := entry{Name: reg.Name}
		if reg.Community {
			e.Community = true
		} else {
			if reg.Stability == featuregate.StabilityUndefined {
				return manifest{}, fmt.Errorf("component %q has undefined stability", reg.Name)
			}
			stability, err := strconv.Unquote(reg.Stability.String())
			if err != nil {
				return manifest{}, fmt.Errorf("component %q: unquoting stability %q: %w", reg.Name, reg.Stability.String(), err)
			}
			e.Stability = stability
		}
		m.Components = append(m.Components, e)
	}
	return m, nil
}

func renderManifest(m manifest) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(header)

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return nil, fmt.Errorf("encoding manifest: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing encoder: %w", err)
	}
	return buf.Bytes(), nil
}

func run(output string) error {
	names := component.AllNames()
	regs := make([]component.Registration, 0, len(names))
	for _, name := range names {
		reg, ok := component.Get(name)
		if !ok {
			continue
		}
		regs = append(regs, reg)
	}

	m, err := buildManifest(regs)
	if err != nil {
		return err
	}
	data, err := renderManifest(m)
	if err != nil {
		return err
	}
	if err := os.WriteFile(output, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", output, err)
	}
	return nil
}

func main() {
	output := flag.String("output", "internal/component/manifest.yaml", "path to write the manifest")
	flag.Parse()

	if err := run(*output); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
