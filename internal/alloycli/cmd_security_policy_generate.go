package alloycli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	alloyast "github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

func securityPolicyGenerateCommand() *cobra.Command {
	sg := &alloySecurityPolicyGenerate{
		configFormat: "alloy",
	}

	cmd := &cobra.Command{
		Use:   "generate [flags] file",
		Short: "Generate the most restrictive security policy for a config",
		Long: `Analyses an Alloy config file and produces a security policy that
allowlists exactly the components and endpoints it uses.

Deploy this policy alongside the config to ensure no other components
or outbound connections can be introduced at runtime.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return sg.Run(args[0])
		},
	}

	cmd.Flags().StringVar(&sg.configFormat, "config.format", sg.configFormat,
		fmt.Sprintf("Format of the config file. Supported: %s", supportedFormatsList()))
	cmd.Flags().StringVarP(&sg.outputPath, "output", "o", "", "Write generated policy to file (default: stdout)")

	return cmd
}

type alloySecurityPolicyGenerate struct {
	configFormat string
	outputPath   string
}

// GeneratedPolicy is the structured result of analysing a config.
// Exported for tests.
type GeneratedPolicy struct {
	// Components is the sorted list of component names used in the config.
	Components []string
	// Endpoints is the sorted list of literal endpoint URLs found.
	Endpoints []string
	// DynamicEndpoints is the count of endpoints that could not be resolved statically.
	DynamicEndpoints int
}

// GeneratePolicy analyses source and returns the tightest possible policy.
func GeneratePolicy(source *alloy_runtime.Source) *GeneratedPolicy {
	reg := component.NewDefaultRegistry(featuregate.StabilityGenerallyAvailable, true)

	// Collect top-level + declare-nested component blocks.
	blocks := source.Components()
	for _, decl := range source.Declares() {
		for _, stmt := range decl.Body {
			if b, ok := stmt.(*alloyast.BlockStmt); ok {
				blocks = append(blocks, b)
			}
		}
	}

	compSet := map[string]bool{}
	epSet := map[string]bool{}
	dynamic := 0

	for _, block := range blocks {
		compName := strings.Join(block.Name, ".")
		compSet[compName] = true

		if registration, err := reg.Get(compName); err == nil {
			argsPtr := registration.CloneArguments()
			scope := vm.NewScope(map[string]any{"module_path": ""})
			if evalErr := vm.New(block.Body).Evaluate(scope, argsPtr); evalErr == nil {
				if ec, ok := argsPtr.(component.EgressComponent); ok {
					spec := ec.EgressSpec()
					for _, u := range spec.Endpoints {
						epSet[u] = true
					}
					if spec.HasDynamic {
						dynamic++
					}
				}
			} else {
				// Expression-based args: count as dynamic if it's an egress component.
				if _, ok := registration.Args.(component.EgressComponent); ok {
					dynamic++
				}
			}
		}
	}

	gp := &GeneratedPolicy{DynamicEndpoints: dynamic}
	for name := range compSet {
		gp.Components = append(gp.Components, name)
	}
	for u := range epSet {
		gp.Endpoints = append(gp.Endpoints, u)
	}
	sort.Strings(gp.Components)
	sort.Strings(gp.Endpoints)
	return gp
}

// policyYAML marshals a GeneratedPolicy to YAML.
// If there are dynamic endpoints the endpoints section is omitted (we can't
// build a correct allowlist without knowing all URLs).
func policyYAML(gp *GeneratedPolicy) (string, error) {
	type section struct {
		Mode string   `yaml:"mode"`
		List []string `yaml:"list"`
	}
	type policy struct {
		Components *section `yaml:"components,omitempty"`
		Endpoints  *section `yaml:"endpoints,omitempty"`
	}

	p := policy{}

	if len(gp.Components) > 0 {
		p.Components = &section{Mode: "allowlist", List: gp.Components}
	}

	if len(gp.Endpoints) > 0 && gp.DynamicEndpoints == 0 {
		p.Endpoints = &section{Mode: "allowlist", List: gp.Endpoints}
	}

	out, err := yaml.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (sg *alloySecurityPolicyGenerate) Run(configPath string) error {
	bold := color.New(color.Bold)
	dim := color.New(color.Faint)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan, color.Bold)
	blue := color.New(color.FgBlue)

	fmt.Println()
	cyan.Printf("🔍 Analysing %s\n", configPath)
	fmt.Println()

	sources, err := loadSourceFiles(configPath, sg.configFormat, false, "")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	source, err := alloy_runtime.ParseSources(sources)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	gp := GeneratePolicy(source)

	// ── Components ──────────────────────────────────────────────────────────
	bold.Printf("🧩  Components found: %d\n", len(gp.Components))
	for _, c := range gp.Components {
		green.Printf("   ✓  %s\n", c)
	}
	fmt.Println()

	// ── Endpoints ────────────────────────────────────────────────────────────
	bold.Printf("🌐  Endpoints found: %d\n", len(gp.Endpoints))
	for _, u := range gp.Endpoints {
		green.Printf("   ✓  %s\n", u)
	}
	if gp.DynamicEndpoints > 0 {
		yellow.Printf("   ⚠️   %d dynamic endpoint(s) — URL resolved at runtime, not included in policy\n", gp.DynamicEndpoints)
	}
	fmt.Println()

	// ── Generate YAML ────────────────────────────────────────────────────────
	yamlOut, err := policyYAML(gp)
	if err != nil {
		return fmt.Errorf("generating policy YAML: %w", err)
	}

	bold.Println("📋  Generated policy (most restrictive):")
	dim.Println(strings.Repeat("─", 50))
	blue.Print(yamlOut)
	dim.Println(strings.Repeat("─", 50))
	fmt.Println()

	if gp.DynamicEndpoints > 0 {
		yellow.Println("⚠️   Endpoint allowlist omitted: config contains expression-based endpoints")
		yellow.Println("    that cannot be resolved statically. Review and add them manually.")
		fmt.Println()
	}

	// ── Write or print ───────────────────────────────────────────────────────
	if sg.outputPath != "" {
		if err := writeFile(sg.outputPath, yamlOut); err != nil {
			return fmt.Errorf("writing policy file: %w", err)
		}
		green.Printf("✅  Policy written to %s\n\n", sg.outputPath)
	} else {
		dim.Printf("💡  Save with:  alloy security-policy generate %s -o policy.yaml\n\n", configPath)
	}

	return nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
