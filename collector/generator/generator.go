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

	// Export components function to a separate package for otelcmd
	// The generator runs from collector directory, so components.go is in the same directory
	if err := exportComponents(dir); err != nil {
		log.Fatalf("failed to export components: %v", err)
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

// exportComponents copies components.go and modifies it to be importable by otelcmd.
// baseDir is the collector directory where components.go is located.
func exportComponents(baseDir string) error {
	componentsPath := filepath.Join(baseDir, "components.go")
	data, err := os.ReadFile(componentsPath)
	if err != nil {
		return fmt.Errorf("read components.go: %w", err)
	}

	content := string(data)

	// Change package from main to components
	content = strings.Replace(content, "package main", "package components", 1)

	// Change function name from components() to Components()
	content = strings.Replace(content, "func components()", "func Components()", 1)

	exportPath := filepath.Join(baseDir, "otelcmd", "internal", "components", "components_impl.go")

	if err := os.MkdirAll(filepath.Dir(exportPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	if err := os.WriteFile(exportPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write export: %w", err)
	}

	return nil
}
