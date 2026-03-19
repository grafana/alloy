package main

import "strings"

const (
	requiredYes = "yes"
	requiredNo  = "no"
)

func printRequired(required bool) string {
	if required {
		return requiredYes
	}
	return requiredNo
}

// schemaIDToRelPath converts a schema ID to a relative filesystem path by
// stripping everything up to and including "internal/component/".
// For example, "grafana/alloy/internal/component/common/net" → "common/net".
// If the marker is not present the ID is returned unchanged.
func schemaIDToRelPath(id string) string {
	const marker = "internal/component/"
	if idx := strings.Index(id, marker); idx >= 0 {
		return id[idx+len(marker):]
	}
	return id
}
