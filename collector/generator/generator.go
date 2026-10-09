package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	//go:embed main_alloy.tpl
	templateMainAlloy []byte
	//go:embed main_windows.tpl
	templateMainWindows []byte
	//go:embed main_native.tpl
	templateMainNative []byte
)

const fileHeader = "// GENERATED CODE: DO NOT EDIT\n\n"

func main() {
	log.Println("Generating Alloy OTel Collector main file...")
	var path, configPath string
	flag.StringVar(&path, "path", "", "path to put generated files")
	flag.StringVar(&configPath, "config", "", "path to the OCB builder config with an optional \"alloy\" section")
	flag.Parse()

	log.Printf("path: %v", path)

	if err := generate(path, configPath); err != nil {
		log.Fatal(err)
	}
}

// generate post-processes the OCB output in path according to the builder
// config at configPath.
func generate(path, configPath string) error {
	cfg, err := readBuilderConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read builder config: %w", err)
	}

	if cfg.nativeOnly() {
		log.Println("No OTel components configured: building Alloy without the OTel Engine")
		if err := writeNativeMain(path); err != nil {
			return fmt.Errorf("failed to write native main file: %w", err)
		}
	} else {
		if err := copyAlloyMainTemplateFromFile(path); err != nil {
			return fmt.Errorf("failed to copy alloy main template: %w", err)
		}

		if err := replaceSectionsOfGeneratedMainFile(path); err != nil {
			return fmt.Errorf("failed to replace command factory: %w", err)
		}

		if err := replaceMainWindows(path); err != nil {
			return fmt.Errorf("failed to replace main_windows.go: %w", err)
		}
	}

	if err := writeAlloyComponents(path, cfg.Alloy); err != nil {
		return fmt.Errorf("failed to generate components_alloy.go: %w", err)
	}
	return nil
}

func copyAlloyMainTemplateFromFile(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create dst dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(path, "main_alloy.go"), append([]byte(fileHeader), templateMainAlloy...), 0o644); err != nil {
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

const (
	allComponentsPackage = "github.com/grafana/alloy/internal/component/all"
	converterPackage     = "github.com/grafana/alloy/internal/converter/enable"
)

// otelOnlyFiles are generated files which wire up the OTel Engine. When
// building Alloy without it, they are emptied rather than deleted, because
// `go generate` fails if files it is about to scan disappear.
var otelOnlyFiles = []string{"components.go", "main_alloy.go", "main_others.go", "main_windows.go"}

const emptyOtelFile = fileHeader + "// Alloy is built without the OTel Engine, so this file is intentionally empty.\n\npackage main\n"

// writeNativeMain replaces the OCB generated main.go with one that only runs
// the Alloy CLI, and empties the files which would import the OTel Engine.
func writeNativeMain(path string) error {
	for _, name := range otelOnlyFiles {
		if err := os.WriteFile(filepath.Join(path, name), []byte(emptyOtelFile), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return os.WriteFile(filepath.Join(path, "main.go"), append([]byte(fileHeader), templateMainNative...), 0o644)
}

// builderConfig is the subset of the OCB builder config read by the generator.
type builderConfig struct {
	// AlloySection is the raw "alloy" section. Its Kind is zero if the config
	// has no such section.
	AlloySection yaml.Node   `yaml:"alloy"`
	Alloy        alloyConfig `yaml:"-"`

	// OTel Collector components built into the OTel Engine.
	Extensions []yaml.Node `yaml:"extensions"`
	Receivers  []yaml.Node `yaml:"receivers"`
	Processors []yaml.Node `yaml:"processors"`
	Exporters  []yaml.Node `yaml:"exporters"`
	Connectors []yaml.Node `yaml:"connectors"`
}

// nativeOnly reports whether Alloy should be built without the OTel Engine.
// This is the case when the config has an "alloy" section but no OTel
// Collector components.
func (c builderConfig) nativeOnly() bool {
	return c.AlloySection.Kind != 0 &&
		len(c.Extensions)+len(c.Receivers)+len(c.Processors)+len(c.Exporters)+len(c.Connectors) == 0
}

// alloyConfig is the "alloy" section of the OCB builder config. OCB ignores
// unknown top-level keys, so this section can live in the same file.
type alloyConfig struct {
	// Components is either the string "all" or a list of native Alloy
	// component names (e.g. "loki.source.file") to include in the build.
	Components yaml.Node `yaml:"components"`
	// Converters controls whether `alloy convert` and running Alloy with a
	// non-Alloy --config.format are supported. Defaults to true.
	Converters *bool `yaml:"converters"`
}

// writeAlloyComponents generates components_alloy.go, which imports the
// native Alloy components (and optionally the config converter) selected in
// the "alloy" section of the builder config.
func writeAlloyComponents(path string, cfg alloyConfig) error {
	imports, err := componentImports(cfg.Components)
	if err != nil {
		return err
	}
	if cfg.Converters == nil || *cfg.Converters {
		imports = append(imports, converterPackage)
	}

	var buf bytes.Buffer
	buf.WriteString(fileHeader)
	buf.WriteString("package main\n")
	if len(imports) > 0 {
		buf.WriteString("\n// Native Alloy components and features selected in the \"alloy\" section of builder-config.yaml.\n")
		buf.WriteString("import (\n")
		for _, imp := range imports {
			fmt.Fprintf(&buf, "\t_ %q\n", imp)
		}
		buf.WriteString(")\n")
	}

	return os.WriteFile(filepath.Join(path, "components_alloy.go"), buf.Bytes(), 0o644)
}

func readBuilderConfig(configPath string) (builderConfig, error) {
	var cfg builderConfig
	if configPath == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.AlloySection.Decode(&cfg.Alloy); err != nil {
		return cfg, fmt.Errorf("parse alloy section: %w", err)
	}
	return cfg, nil
}

// componentImports returns the sorted list of packages to import for the
// selected components.
func componentImports(node yaml.Node) ([]string, error) {
	switch node.Kind {
	case 0: // Not set.
		return []string{allComponentsPackage}, nil
	case yaml.ScalarNode:
		if node.Value != "all" {
			return nil, fmt.Errorf(`alloy.components must be "all" or a list of component names, got %q`, node.Value)
		}
		return []string{allComponentsPackage}, nil
	case yaml.SequenceNode:
		var names []string
		if err := node.Decode(&names); err != nil {
			return nil, fmt.Errorf("decode alloy.components: %w", err)
		}
		return resolveComponentPackages(names)
	default:
		return nil, fmt.Errorf(`alloy.components must be "all" or a list of component names`)
	}
}

// resolveComponentPackages maps component names to the packages which
// register them. Several components may be registered by the same package.
func resolveComponentPackages(names []string) ([]string, error) {
	known, err := lookupComponentPackages()
	if err != nil {
		return nil, err
	}

	var (
		pkgs    []string
		unknown []string
	)
	for _, name := range names {
		pkg, ok := known[name]
		if !ok {
			unknown = append(unknown, unknownComponentMessage(name, known))
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown components in alloy.components:\n  %s", strings.Join(unknown, "\n  "))
	}

	slices.Sort(pkgs)
	return slices.Compact(pkgs), nil
}

// lookupComponentPackages returns a map of component names to the packages
// which register them. It is a variable so tests can replace it.
var lookupComponentPackages = knownComponentPackages

// knownComponentPackages runs the componentpkgs helper, which compiles every
// native Alloy component and reports which package registers each of them.
func knownComponentPackages() (map[string]string, error) {
	log.Println("Resolving native Alloy component packages...")

	cmd := exec.Command("go", "run", "github.com/grafana/alloy/otel_engine/generator/componentpkgs")
	cmd.Stderr = os.Stderr
	// Generation runs with CGO_ENABLED=0, but some components only build with
	// cgo on some platforms. Fall back to Go's default, like the Alloy build.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "CGO_ENABLED=")
	})
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run componentpkgs: %w", err)
	}

	var known map[string]string
	if err := json.Unmarshal(out, &known); err != nil {
		return nil, fmt.Errorf("parse componentpkgs output: %w", err)
	}
	return known, nil
}

func unknownComponentMessage(name string, known map[string]string) string {
	namespace, _, _ := strings.Cut(name, ".")

	var similar []string
	for k := range known {
		if strings.HasPrefix(k, namespace+".") {
			similar = append(similar, k)
		}
	}
	if len(similar) == 0 {
		return fmt.Sprintf("%q", name)
	}
	slices.Sort(similar)
	return fmt.Sprintf("%q (components in the %q namespace: %s)", name, namespace, strings.Join(similar, ", "))
}
