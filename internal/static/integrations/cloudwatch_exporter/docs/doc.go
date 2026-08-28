package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	yaceConf "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
)

//go:embed template.md
var docTemplate string

const servicesListReplacement = "{{SERVICE_LIST}}"

// main generates shared documentation containing all AWS services supported in CloudWatch exporter discovery jobs.
func main() {
	argsWithoutProgram := os.Args[1:]
	if len(argsWithoutProgram) != 2 || argsWithoutProgram[0] != "sync" {
		log.Printf("Usage: %s sync <file>\n", os.Args[0])
		os.Exit(1)
	}

	fileToSync := argsWithoutProgram[1]
	changed, err := syncServicesDoc(fileToSync, generateServicesDoc())
	if err != nil {
		log.Printf("Failed to sync %s: %s\n", fileToSync, err)
		os.Exit(1)
	}
	if changed {
		log.Printf("Updated %s\n", fileToSync)
		return
	}
	log.Printf("%s is already up to date\n", fileToSync)
}

func syncServicesDoc(path string, expectedDoc string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read file: %w", err)
	}
	if string(contents) == expectedDoc {
		return false, nil
	}

	mode := os.FileMode(0o644)
	if fileInfo, err := os.Stat(path); err == nil {
		mode = fileInfo.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat file: %w", err)
	}
	if err := os.WriteFile(path, []byte(expectedDoc), mode); err != nil {
		return false, fmt.Errorf("write file: %w", err)
	}
	return true, nil
}

func generateServicesDoc() string {
	var sb strings.Builder

	slices.SortFunc(yaceConf.SupportedServices, func(i, j yaceConf.ServiceConfig) int {
		return strings.Compare(strings.ToLower(i.Namespace), strings.ToLower(j.Namespace))
	})

	for i, supportedSvc := range yaceConf.SupportedServices {
		fmt.Fprintf(&sb, "* Namespace: `%s`", supportedSvc.Namespace) //nolint:errcheck // strings.Builder write cannot fail
		if i < len(yaceConf.SupportedServices)-1 {
			sb.WriteString("\n")
		}
	}

	doc := strings.Replace(docTemplate, servicesListReplacement, sb.String(), 1)
	return doc
}
