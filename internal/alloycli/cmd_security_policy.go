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

// checkResult holds per-component findings.
type checkResult struct {
	name             string // e.g. "remote.http"
	label            string // e.g. "exfil"
	componentBlocked bool
	blockReason      string
	endpoints        []endpointResult
}

type endpointResult struct {
	url       string
	blocked   bool
	reason    string
	dynamic   bool // cannot be verified statically
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

	sources, err := loadSourceFiles(configPath, sc.configFormat, false, "")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	source, err := alloy_runtime.ParseSources(sources)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	reg := component.NewDefaultRegistry(featuregate.StabilityGenerallyAvailable, true)

	// Collect all component blocks: top-level + those nested inside declare blocks.
	allComponentBlocks := source.Components()
	for _, decl := range source.Declares() {
		for _, stmt := range decl.Body {
			if b, ok := stmt.(*alloyast.BlockStmt); ok {
				allComponentBlocks = append(allComponentBlocks, b)
			}
		}
	}

	var results []checkResult
	violations := 0
	dynamic := 0

	for _, block := range allComponentBlocks {
		compName := strings.Join(block.Name, ".")
		label := block.Label
		r := checkResult{name: compName, label: label}

		// Phase 1: component gate
		if err := policy.CheckComponent(compName); err != nil {
			r.componentBlocked = true
			r.blockReason = err.Error()
			violations++
		}

		// Phase 2: endpoint gate (best-effort on literal values)
		if !r.componentBlocked {
			if registration, regErr := reg.Get(compName); regErr == nil {
				argsPtr := registration.CloneArguments()
				scope := vm.NewScope(map[string]any{"module_path": ""})
				if evalErr := vm.New(block.Body).Evaluate(scope, argsPtr); evalErr == nil {
					// Args decoded — check if it declares egress endpoints
					if ec, ok := argsPtr.(component.EgressComponent); ok {
						spec := ec.EgressSpec()
						for _, u := range spec.Endpoints {
							ep := endpointResult{url: u}
							if endpointErr := policy.CheckEndpoint(u); endpointErr != nil {
								ep.blocked = true
								ep.reason = endpointErr.Error()
								violations++
							}
							r.endpoints = append(r.endpoints, ep)
						}
						if spec.HasDynamic {
							r.endpoints = append(r.endpoints, endpointResult{dynamic: true})
							dynamic++
						}
					}
				} else {
					// Could not evaluate (expression-based args) — flag as unverifiable if EgressComponent
					if _, ok := registration.Args.(component.EgressComponent); ok {
						r.endpoints = append(r.endpoints, endpointResult{dynamic: true})
						dynamic++
					}
				}
			}
		}

		results = append(results, r)
	}

	// Print component results
	bold.Println("📋  Component & Endpoint Policy")
	fmt.Println()

	for _, r := range results {
		qualifier := r.name
		if r.label != "" {
			qualifier = fmt.Sprintf("%s %q", r.name, r.label)
		}
		if r.componentBlocked {
			red.Printf("   ❌  %s\n", qualifier)
			dim.Printf("       %s\n", r.blockReason)
		} else {
			green.Printf("   ✅  %s\n", qualifier)
		}
		for _, ep := range r.endpoints {
			if ep.dynamic {
				yellow.Printf("       ⚠️   endpoint cannot be verified statically — enforced at runtime\n")
			} else if ep.blocked {
				red.Printf("       ❌  endpoint %q\n", ep.url)
				dim.Printf("           %s\n", ep.reason)
			} else {
				green.Printf("       🌐  endpoint %q\n", ep.url)
				dim.Printf("           allowed by endpoint policy\n")
			}
		}
	}

	// Config blocks (informational)
	configBlocks := source.Configs()
	if len(configBlocks) > 0 {
		fmt.Println()
		bold.Println("📦  Config Blocks")
		fmt.Println()
		for _, b := range configBlocks {
			name := b.GetBlockName()
			label := b.Label
			qualifier := name
			if label != "" {
				qualifier = fmt.Sprintf("%s %q", name, label)
			}
			dim.Printf("   ℹ️   %s  (runtime-only validation)\n", qualifier)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	switch {
	case violations > 0:
		red.Printf("❌  %d policy violation(s) — this config would be REJECTED at runtime.\n", violations)
		if dynamic > 0 {
			yellow.Printf("⚠️   %d endpoint(s) could not be verified statically.\n", dynamic)
		}
		fmt.Println()
		os.Exit(1)
	case dynamic > 0:
		green.Println("✅  No violations found.")
		yellow.Printf("⚠️   %d endpoint(s) could not be verified statically — check runtime logs.\n", dynamic)
	default:
		green.Println("✅  No violations found. Config is consistent with the security policy.")
	}

	fmt.Println()
	return nil
}
