package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	//go:embed main_alloy.tpl
	templateMainAlloy []byte
	//go:embed main_windows.tpl
	templateMainWindows []byte
)

func main() {
	log.Println("Generating Alloy OTel Collector main file...")
	var path string
	flag.StringVar(&path, "path", "", "path to put generated files")
	flag.Parse()

	log.Printf("path: %v", path)

	if err := copyAlloyMainTemplateFromFile(path); err != nil {
		log.Fatalf("failed to copy alloy main template: %v", err)
	}

	if err := replaceSectionsOfGeneratedMainFile(path); err != nil {
		log.Fatalf("failed to replace command factory: %v", err)
	}

	if err := replaceMainWindows(path); err != nil {
		log.Fatalf("failed to replace main_windows.go: %v", err)
	}
}

// copyAlloyMainTemplateFromFile copies the template from templatePath to dstPath.
func copyAlloyMainTemplateFromFile(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create dst dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(path, "main_alloy.go"), templateMainAlloy, 0o644); err != nil {
		return fmt.Errorf("write template to %s: %w", path, err)
	}
	return nil
}

func replaceSectionsOfGeneratedMainFile(path string) error {
	main := filepath.Join(path, "main.go")

	data, err := os.ReadFile(main)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	lines, err = replaceCmdFactory(lines)

	if err != nil {
		return fmt.Errorf("error replacing command factory in %s: %w", path, err)
	}

	lines, err = addReleasePleaseVersioning(lines)

	if err != nil {
		return fmt.Errorf("error setting collector veresion in %s: %w", path, err)
	}

	newContent := strings.Join(lines, "\n")
	fi, err := os.Stat(main)
	var mode os.FileMode = 0o644
	if err == nil {
		mode = fi.Mode()
	}

	if err := os.WriteFile(main, []byte(newContent), mode); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

// replaceCmdFactory processes incoming lines and will search for
// "cmd := otelcol.NewCommand(params)" and replace it with
// "cmd := newAlloyCommand(params)"
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

// replaceCmdFactory processes incoming lines and will search for
// `Version: "..."` and replace it with
// `Version: CollectorVersion()`
func addReleasePleaseVersioning(lines []string) ([]string, error) {
	versionPattern := regexp.MustCompile(`^(\s+Version:\s+)"[^"]+"(,)(\s*//.*)?$`)
	versionReplaced := false
	for i, line := range lines {
		if matches := versionPattern.FindStringSubmatch(line); matches != nil {
			lines[i] = matches[1] + `CollectorVersion()` + matches[2]
			versionReplaced = true
			break
		}
	}

	if !versionReplaced {
		return nil, fmt.Errorf("version field not found")
	}

	return lines, nil
}

func replaceMainWindows(path string) error {
	if err := os.WriteFile(filepath.Join(path, "main_windows.go"), []byte(templateMainWindows), 0o644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}
