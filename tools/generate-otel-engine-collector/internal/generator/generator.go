package generator

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed main_alloy.tpl
var mainAlloyTemplate []byte

// clearGeneratedFiles removes main*.go, components.go, go.mod, and go.sum from collectorDir
// so we generate from scratch.
func clearGeneratedFiles(collectorDir string) error {
	mainGlob := filepath.Join(collectorDir, "main*.go")
	matches, err := filepath.Glob(mainGlob)
	if err != nil {
		return err
	}
	for _, p := range matches {
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	for _, name := range []string{"components.go", "go.mod", "go.sum"} {
		p := filepath.Join(collectorDir, name)
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	return nil
}

// Generate clears existing generated files, runs the OTel builder in collectorDir, then go mod tidy,
// then post-processes main.go and writes main_alloy.go using the embedded template.
func Generate(collectorDir, builderVersion string) error {
	configPath := filepath.Join(collectorDir, "builder-config.yaml")
	if _, err := os.Stat(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("collector config not found at %s: %w", configPath, err)
	}

	if err := clearGeneratedFiles(collectorDir); err != nil {
		return fmt.Errorf("clear generated files: %w", err)
	}

	builderCmd := exec.Command("go", "run", "go.opentelemetry.io/collector/cmd/builder@"+builderVersion,
		"--config", "builder-config.yaml",
		"--skip-compilation",
	)
	builderCmd.Dir = collectorDir
	builderCmd.Env = append(os.Environ(), "GOOS=", "GOARCH=") // avoid cross-compiling the builder
	builderCmd.Stdout = os.Stdout
	builderCmd.Stderr = os.Stderr
	if err := builderCmd.Run(); err != nil {
		return fmt.Errorf("otel builder: %w", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = collectorDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	mainPath := filepath.Join(collectorDir, "main.go")
	alloyMainPath := filepath.Join(collectorDir, "main_alloy.go")

	if err := writeAlloyMainFromTemplate(alloyMainPath); err != nil {
		return fmt.Errorf("write main_alloy.go: %w", err)
	}

	if err := patchGeneratedMain(mainPath); err != nil {
		return fmt.Errorf("patch main.go: %w", err)
	}

	return nil
}

func writeAlloyMainFromTemplate(dstPath string) error {
	header := []byte("// GENERATED CODE: DO NOT EDIT\n")
	data := append(header, mainAlloyTemplate...)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0o644)
}

func patchGeneratedMain(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	lines, err = replaceCmdFactory(lines)
	if err != nil {
		return fmt.Errorf("replace command factory: %w", err)
	}
	lines, err = addReleasePleaseVersioning(lines)
	if err != nil {
		return fmt.Errorf("replace version: %w", err)
	}
	newContent := strings.Join(lines, "\n")
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(filePath); err == nil {
		mode = fi.Mode()
	}
	return os.WriteFile(filePath, []byte(newContent), mode)
}

func replaceCmdFactory(lines []string) ([]string, error) {
	const target = "cmd := otelcol.NewCommand(params)"
	const replacement = "cmd := newAlloyCommand(params)"
	for i, line := range lines {
		if strings.Contains(line, target) {
			lines[i] = strings.Replace(line, target, replacement, 1)
			return lines, nil
		}
	}
	return nil, fmt.Errorf("target line %q not found", target)
}

func addReleasePleaseVersioning(lines []string) ([]string, error) {
	versionPattern := regexp.MustCompile(`^(\s+Version:\s+)"[^"]+"(,)(\s*//.*)?$`)
	for i, line := range lines {
		if matches := versionPattern.FindStringSubmatch(line); matches != nil {
			lines[i] = matches[1] + `CollectorVersion()` + matches[2]
			return lines, nil
		}
	}
	return nil, fmt.Errorf("Version field not found")
}
