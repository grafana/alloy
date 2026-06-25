package alloycli

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/securitypolicy"
	alloyast "github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/vm"
)

func securityPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security-policy",
		Short: "Security policy tools",
	}
	cmd.AddCommand(securityPolicyCheckCommand())
	return cmd
}

func securityPolicyCheckCommand() *cobra.Command {
	sc := &alloySecurityPolicyCheck{
		configFormat: "alloy",
	}

	cmd := &cobra.Command{
		Use:          "check [flags] file",
		Short:        "Check an Alloy config against a security policy",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return sc.Run(args[0])
		},
	}

	cmd.Flags().StringVar(&sc.policyPath, "security-policy", "", "Path to the security policy YAML file (required)")
	_ = cmd.MarkFlagRequired("security-policy")
	cmd.Flags().StringVar(&sc.configFormat, "config.format", sc.configFormat,
		fmt.Sprintf("Format of the config file. Supported: %s", supportedFormatsList()))

	return cmd
}

type alloySecurityPolicyCheck struct {
	policyPath   string
	configFormat string
}

// componentFinding holds the result of checking one component block.
type componentFinding struct {
	// Name is the component type, e.g. "remote.http".
	Name string
	// Label is the block label, e.g. "exfil".
	Label string
	// ComponentViolation is set when the component itself is denied by policy.
	ComponentViolation string
	// EndpointFindings is the per-endpoint results (only populated when the
	// component passes the component gate and implements EgressComponent).
	EndpointFindings []endpointFinding
}

type endpointFinding struct {
	URL       string
	Violation string // non-empty = denied; empty = allowed
	Dynamic   bool   // true when URL cannot be resolved statically
}

// PolicyCheckReport is the output of CheckConfig — pure data, no I/O.
// It is exported so tests can inspect it directly.
type PolicyCheckReport struct {
	Components   []componentFinding
	ConfigBlocks []string // informational: names of import.http etc. config blocks
	Violations   int
	Dynamic      int
}

// CheckConfig loads and checks configPath against policy. Separated from
// rendering so tests can call it without touching stdout.
func CheckConfig(policy *securitypolicy.SecurityPolicy, configPath, configFormat string) (*PolicyCheckReport, error) {
	sources, err := loadSourceFiles(configPath, configFormat, false, "")
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	source, err := alloy_runtime.ParseSources(sources)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return CheckSource(policy, source), nil
}

// CheckSource checks an already-parsed source against policy. Exported for
// testing without filesystem access.
func CheckSource(policy *securitypolicy.SecurityPolicy, source *alloy_runtime.Source) *PolicyCheckReport {
	report := &PolicyCheckReport{}

	for _, b := range source.Configs() {
		q := b.GetBlockName()
		if b.Label != "" {
			q = fmt.Sprintf("%s %q", q, b.Label)
		}
		report.ConfigBlocks = append(report.ConfigBlocks, q)
	}

	reg := component.NewDefaultRegistry(featuregate.StabilityGenerallyAvailable, true)

	// Collect top-level components + those nested inside declare blocks.
	blocks := source.Components()
	for _, decl := range source.Declares() {
		for _, stmt := range decl.Body {
			if b, ok := stmt.(*alloyast.BlockStmt); ok {
				blocks = append(blocks, b)
			}
		}
	}

	for _, block := range blocks {
		compName := strings.Join(block.Name, ".")
		f := componentFinding{Name: compName, Label: block.Label}

		if err := policy.CheckComponent(compName); err != nil {
			f.ComponentViolation = err.Error()
			report.Violations++
			report.Components = append(report.Components, f)
			continue
		}

		// Endpoint gate: best-effort on literal-value configs.
		if registration, regErr := reg.Get(compName); regErr == nil {
			argsPtr := registration.CloneArguments()
			scope := vm.NewScope(map[string]any{"module_path": ""})
			if evalErr := vm.New(block.Body).Evaluate(scope, argsPtr); evalErr == nil {
				if ec, ok := argsPtr.(component.EgressComponent); ok {
					spec := ec.EgressSpec()
					for _, u := range spec.Endpoints {
						ef := endpointFinding{URL: u}
						if endpointErr := policy.CheckEndpoint(u); endpointErr != nil {
							ef.Violation = endpointErr.Error()
							report.Violations++
						}
						f.EndpointFindings = append(f.EndpointFindings, ef)
					}
					if spec.HasDynamic {
						f.EndpointFindings = append(f.EndpointFindings, endpointFinding{Dynamic: true})
						report.Dynamic++
					}
				}
			} else {
				// Expression-based args: if the type implements EgressComponent, warn.
				if _, ok := registration.Args.(component.EgressComponent); ok {
					f.EndpointFindings = append(f.EndpointFindings, endpointFinding{Dynamic: true})
					report.Dynamic++
				}
			}
		}

		report.Components = append(report.Components, f)
	}

	return report
}

func (sc *alloySecurityPolicyCheck) Run(configPath string) error {
	bold := color.New(color.Bold)
	dim := color.New(color.Faint)
	red := color.New(color.FgRed, color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan, color.Bold)

	fmt.Println()
	cyan.Printf("🔍 Checking %s against %s\n", configPath, sc.policyPath)
	fmt.Println()

	policy, err := securitypolicy.LoadFromFile(sc.policyPath)
	if err != nil {
		return fmt.Errorf("loading policy: %w", err)
	}

	report, err := CheckConfig(policy, configPath, sc.configFormat)
	if err != nil {
		return err
	}

	bold.Println("📋  Component & Endpoint Policy")
	fmt.Println()

	for _, f := range report.Components {
		qualifier := f.Name
		if f.Label != "" {
			qualifier = fmt.Sprintf("%s %q", f.Name, f.Label)
		}
		if f.ComponentViolation != "" {
			red.Printf("   ❌  %s\n", qualifier)
			dim.Printf("       %s\n", f.ComponentViolation)
		} else {
			green.Printf("   ✅  %s\n", qualifier)
		}
		for _, ep := range f.EndpointFindings {
			switch {
			case ep.Dynamic:
				yellow.Printf("       ⚠️   endpoint cannot be verified statically — enforced at runtime\n")
			case ep.Violation != "":
				red.Printf("       ❌  endpoint %q\n", ep.URL)
				dim.Printf("           %s\n", ep.Violation)
			default:
				green.Printf("       🌐  endpoint %q — allowed\n", ep.URL)
			}
		}
	}

	if len(report.ConfigBlocks) > 0 {
		fmt.Println()
		bold.Println("📦  Config Blocks")
		fmt.Println()
		for _, name := range report.ConfigBlocks {
			dim.Printf("   ℹ️   %s  (runtime-only validation)\n", name)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	switch {
	case report.Violations > 0:
		red.Printf("❌  %d policy violation(s) — this config would be REJECTED at runtime.\n", report.Violations)
		if report.Dynamic > 0 {
			yellow.Printf("⚠️   %d endpoint(s) could not be verified statically.\n", report.Dynamic)
		}
		fmt.Println()
		os.Exit(1)
	case report.Dynamic > 0:
		green.Println("✅  No violations found.")
		yellow.Printf("⚠️   %d endpoint(s) could not be verified statically — check runtime logs.\n", report.Dynamic)
	default:
		green.Println("✅  No violations found. Config is consistent with the security policy.")
	}

	fmt.Println()
	return nil
}
