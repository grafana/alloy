package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strings"

	yaceConf "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/config"
)

//go:embed template.md
var docTemplate string

const servicesListReplacement = "{{SERVICE_LIST}}"

// main is used for programmatically generating a documentation section containing all AWS services supported in cloudwatch
// exporter discovery jobs.
func main() {
	programName := os.Args[0]
	argsWithoutProgram := os.Args[1:]
	if len(argsWithoutProgram) < 1 {
		log.Println("Missing arguments: generate OR check <file>")
		os.Exit(1)
	}
	doc := generateServicesDocSection()
	switch argsWithoutProgram[0] {
	case "generate":
		fmt.Println(doc)
	case "check":
		if len(argsWithoutProgram) < 2 {
			log.Println("Missing arguments: check <file>")
			os.Exit(1)
		}
		fileToCheck := argsWithoutProgram[1]
		if err := checkServicesDocSection(fileToCheck, doc); err != nil {
			log.Printf("Check failed: %s\n", err)
			log.Printf("Try updating %s with the services section produced by `%s generate`\n", fileToCheck, programName)
			os.Exit(1)
		}
		log.Println("Check successful!")
	default:
		log.Printf("Unknown command: %s\n", argsWithoutProgram[0])
		os.Exit(1)
	}
}

func checkServicesDocSection(path string, expectedDoc string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file to check: %w", err)
	}
	if !strings.Contains(string(contents), strings.TrimRight(expectedDoc, "\n")) {
		return fmt.Errorf("doc has no services section, or is out of date")
	}
	return nil
}

func generateServicesDocSection() string {
	var sb strings.Builder
	for _, supportedSvc := range yaceConf.SupportedServices {
		sb.WriteString(
			fmt.Sprintf("- Namespace: `%s` or Alias: `%s`\n", supportedSvc.Namespace, supportedSvc.Alias),
		)
	}
	doc := strings.Replace(docTemplate, servicesListReplacement, sb.String(), 1)
	return doc
}
