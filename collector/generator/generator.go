package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

var (
	//go:embed main_alloy.tpl
	templateMainAlloy []byte
	//go:embed main_windows.tpl
	templateMainWindows []byte
)

const fileHeader = "// GENERATED CODE: DO NOT EDIT\n\n"

func main() {
	log.Println("Generating Alloy OTel Collector main file...")
	var path string
	flag.StringVar(&path, "path", "", "path to put generated files")
	flag.Parse()

	log.Printf("path: %v", path)

	alloyComponentsPath := filepath.Join(path, "alloy-components.yaml")
	allPackagePath := filepath.Join(path, "..", "internal", "component", "all")
	if err := generate(path, alloyComponentsPath, allPackagePath); err != nil {
		log.Fatal(err)
	}
}

// generate post-processes the OCB output in path, and generates the package
// at allPackagePath, according to the native components listed at
// alloyComponentsPath.
func generate(path, alloyComponentsPath, allPackagePath string) error {
	cfg, err := readAlloyConfig(alloyComponentsPath)
	if err != nil {
		return fmt.Errorf("failed to read alloy components config: %w", err)
	}

	if err := writeAllPackage(allPackagePath, cfg); err != nil {
		return fmt.Errorf("failed to generate the all package: %w", err)
	}

	if err := writeAlloyMain(path, cfg); err != nil {
		return fmt.Errorf("failed to write alloy main template: %w", err)
	}

	if err := replaceSectionsOfGeneratedMainFile(path); err != nil {
		return fmt.Errorf("failed to replace command factory: %w", err)
	}

	if err := replaceMainWindows(path); err != nil {
		return fmt.Errorf("failed to replace main_windows.go: %w", err)
	}
	return nil
}

// alloyConfig is the content of alloy-components.yaml, which lists the native
// Alloy components. It is separate from the OCB builder config because OCB
// doesn't know about native components.
type alloyConfig struct {
	// Components are the names of the native Alloy components to include in
	// the build. It must be set; an empty list includes no components.
	Components *[]string `yaml:"components"`
	// Converters controls whether `alloy convert` and running Alloy with a
	// non-Alloy --config.format are supported. Defaults to true.
	Converters bool `yaml:"converters"`
}

// readAlloyConfig reads alloy-components.yaml.
func readAlloyConfig(path string) (alloyConfig, error) {
	cfg := alloyConfig{Converters: true}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	// Fail on unknown keys, so a typo doesn't silently change the build.
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	// An empty value, for example after commenting out every entry, must not
	// be mistaken for a list.
	if cfg.Components == nil {
		return cfg, errors.New("components must be set to a list; use [] for no components")
	}
	return cfg, nil
}

const componentPackage = "github.com/grafana/alloy/internal/component"

// writeAllPackage generates all.go in dir, which imports the components
// listed in alloy-components.yaml. The component "a.b.c" is defined in the
// package internal/component/a/b/c, which alloylint checks.
func writeAllPackage(dir string, cfg alloyConfig) error {
	names := slices.Clone(*cfg.Components)
	slices.Sort(names)
	names = slices.Compact(names)

	var unknown []string
	for _, name := range names {
		rel := strings.ReplaceAll(name, ".", "/")
		if fi, err := os.Stat(filepath.Join(dir, "..", filepath.FromSlash(rel))); err != nil || !fi.IsDir() {
			unknown = append(unknown, fmt.Sprintf("%q (no package internal/component/%s)", name, rel))
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown components in alloy-components.yaml:\n  %s", strings.Join(unknown, "\n  "))
	}

	var buf bytes.Buffer
	buf.WriteString("// Code generated from collector/alloy-components.yaml by collector/generator. DO NOT EDIT.\n\n")
	buf.WriteString("// Package all imports the native Alloy components listed in\n")
	buf.WriteString("// collector/alloy-components.yaml.\n")
	buf.WriteString("package all\n")
	if len(names) > 0 {
		buf.WriteString("\nimport (\n")
		for _, name := range names {
			fmt.Fprintf(&buf, "\t_ %q // Import %s\n", componentPackage+"/"+strings.ReplaceAll(name, ".", "/"), name)
		}
		buf.WriteString(")\n")
	}

	// Align the comments, like gofmt does.
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format all.go: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "all.go"), src, 0o644)
}

var mainAlloyTemplate = template.Must(template.New("main_alloy.tpl").Parse(string(templateMainAlloy)))

// writeAlloyMain generates main_alloy.go, which also imports the native
// components and, unless they're disabled, the config converters.
func writeAlloyMain(path string, cfg alloyConfig) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create dst dir: %w", err)
	}

	buf := bytes.NewBufferString(fileHeader)
	if err := mainAlloyTemplate.Execute(buf, cfg); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(path, "main_alloy.go"), buf.Bytes(), 0o644); err != nil {
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
		return fmt.Errorf("error setting collector version in %s: %w", path, err)
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
	if err := os.WriteFile(filepath.Join(path, "main_windows.go"), append([]byte(fileHeader), templateMainWindows...), 0o644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}
