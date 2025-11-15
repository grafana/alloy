package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	log.Println("Generating Alloy OTel Collector main file...")
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get working directory: %v", err)
	}
	log.Printf("working dir: %v", dir)
	otelGeneratedMain := os.Args[2]
	alloyMain := os.Args[3]
	log.Printf("otelGeneratedMain: %v", otelGeneratedMain)
	log.Printf("alloyMain: %v", alloyMain)

	templatePath := filepath.Join(dir, "generator", "main_allloy.tpl")
	if err := copyAlloyMainTemplateFromFile(templatePath, alloyMain); err != nil {
		log.Fatalf("failed to copy alloy main template: %v", err)
	}

	if err := replaceCmdFactory(otelGeneratedMain); err != nil {
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
	if err := os.WriteFile(dstPath, data, 0o644); err != nil {
		return fmt.Errorf("write template to %s: %w", dstPath, err)
	}
	return nil
}

// replaceCmdFactory reads the file at filePath, finds the line containing
// "cmd := otelcol.NewCommand(params)", and replaces it with
// "cmd := newAlloyCommand(params)" while preserving the line's indentation.
func replaceCmdFactory(filePath string) error {
	const target = "cmd := otelcol.NewCommand(params)"
	const replacement = "cmd := newAlloyCommand(params)"

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	replaced := false
	for i, line := range lines {
		if strings.Contains(line, target) {
			lines[i] = strings.Replace(line, target, replacement, 1)
			replaced = true
			break
		}
	}

	if !replaced {
		return fmt.Errorf("target line not found in %s", filePath)
	}

	newContent := strings.Join(lines, "\n")

	fi, err := os.Stat(filePath)
	var mode os.FileMode = 0o644
	if err == nil {
		mode = fi.Mode()
	}

	if err := os.WriteFile(filePath, []byte(newContent), mode); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
