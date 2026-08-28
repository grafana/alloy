package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionString string

func CollectorVersion() string {
	return CollectorVersionFromFile(versionString)
}

func CollectorVersionFromFile(fileContents string) string {
	versionStringWithoutComment := strings.Split(fileContents, "#")[0]
	return "v" + strings.TrimSpace(versionStringWithoutComment)
}
