package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	log.Println("Generating Alloy OTel Collector main file...")

	otelGeneratedMain := flag.String("main-path", "", "Path to the OTel-generated main.go file")
	alloyMain := flag.String("main-alloy-path", "", "Path to the generated main_alloy.go file")
	flag.Parse()

	log.Printf("otelGeneratedMain: %v", *otelGeneratedMain)
	log.Printf("alloyMain: %v", *alloyMain)

	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}

	templatePath := filepath.Join(dir, "generator", "main_alloy.tpl")
	if err := copyAlloyMainTemplateFromFile(templatePath, *alloyMain); err != nil {
		log.Fatalf("failed to copy alloy main template: %v", err)
	}

	if err := replaceSectionsOfGeneratedMainFile(*otelGeneratedMain); err != nil {
		log.Fatalf("failed to replace command factory: %v", err)
	}
}

// copyAlloyMainTemplateFromFile copies the template from templatePath to dstPath.
func copyAlloyMainTemplateFromFile(templatePath, dstPath string) error {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", templatePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create dst dir: %w", err)
	}

	withGeneratedWarningHeader := append([]byte("// GENERATED CODE: DO NOT EDIT\n"), data...)

	if err := os.WriteFile(dstPath, withGeneratedWarningHeader, 0o644); err != nil {
		return fmt.Errorf("write template to %s: %w", dstPath, err)
	}
	return nil
}

func replaceSectionsOfGeneratedMainFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	lines, err = replaceCmdFactory(lines)

	if err != nil {
		return fmt.Errorf("error replacing command factory in %s: %w", filePath, err)
	}

	lines, err = addReleasePleaseVersioning(lines)

	if err != nil {
		return fmt.Errorf("error adding release-please version comment in %s: %w", filePath, err)
	}

	newContent := strings.Join(lines, "\n")
	fi, err := os.Stat(filePath)
	var mode os.FileMode = 0o644
	if err == nil {
		mode = fi.Mode()
	}

	if err := os.WriteFile(filePath, []byte(newContent), mode); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

// replaceCmdFactory reads the file at filePath, finds the line containing
// "cmd := otelcol.NewCommand(params)", and replaces it with
// "cmd := newAlloyCommand(params)" while preserving the line's indentation.
func replaceCmdFactory(lines []string) ([]string, error) {
	const target = "cmd := otelcol.NewCommand(params)"
	const replacement = "cmd := newAlloyCommand(params)"

	replaced := false
	for i, line := range lines {
		if strings.Contains(line, target) {
			lines[i] = strings.Replace(line, target, replacement, 1)
			replaced = true
			break
		}
	}

	if !replaced {
		return nil, fmt.Errorf("target line not found")
	}

	return lines, nil
}

// addReleasePleaseVersioning reads the file at filePath, finds the Version field
// in the component.BuildInfo struct, and adds the x-release-please-version comment.
func addReleasePleaseVersioning(lines []string) ([]string, error) {
	const commentMarker = " // x-release-please-version"

	replaced := false

	// Pattern to match: Version:     "vX.Y.Z", (matches any version string)
	versionPattern := regexp.MustCompile(`^(\s+Version:\s+"[^"]+",)(\s*//.*)?$`)

	for i, line := range lines {
		if matches := versionPattern.FindStringSubmatch(line); matches != nil {
			lines[i] = matches[1] + commentMarker
			replaced = true
			break
		}
	}

	if !replaced {
		return nil, fmt.Errorf("Version field not found")
	}

	return lines, nil
}
