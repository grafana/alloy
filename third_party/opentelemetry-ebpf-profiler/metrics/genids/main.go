// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type metricDef struct {
	Description string `json:"description"`
	MetricType  string `json:"type"`
	Name        string `json:"name"`
	FieldName   string `json:"field"`
	ID          uint32 `json:"id"`
	Obsolete    bool   `json:"obsolete"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <metrics.json> <output.go>\n", os.Args[0])
		os.Exit(1)
	}

	input, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v", os.Args[1], err)
		os.Exit(1)
	}

	var metricDefs []metricDef
	if err = json.Unmarshal(input, &metricDefs); err != nil {
		fmt.Fprintf(os.Stderr, "Error unmarshaling: %v", err)
		os.Exit(1)
	}

	var output bytes.Buffer
	output.WriteString(
		"// Code generated from metrics.json. DO NOT EDIT.\n" +
			"\n" +
			"package metrics\n" +
			"\n" +
			"// To add a new metric append an entry to metrics.json. ONLY APPEND !\n" +
			"// Then run 'make generate' from the top directory.\n" +
			"\n" +
			"// Below are the different metric IDs that we currently implement.\n" +
			"const (\n")

	for _, m := range metricDefs {
		if m.Obsolete {
			continue
		}

		output.WriteString(
			fmt.Sprintf("\n\t// %s\n\tID%s = %d\n",
				m.Description, m.Name, m.ID))
	}

	output.WriteString(
		"\n\t// max number of ID values, keep this as *last entry*\n" +
			fmt.Sprintf("\tIDMax = %d\n)\n", len(metricDefs)))

	if err = os.WriteFile(os.Args[2], output.Bytes(), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}
}
