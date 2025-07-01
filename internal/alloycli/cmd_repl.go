package alloycli

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/featuregate"
	// Install Components
	// _ "github.com/grafana/alloy/internal/component/all"
)

func replCommand() *cobra.Command {
	r := &alloyRepl{
		httpAddr: "127.0.0.1:12345",
		// storagePath:    "data-alloy/",
		minStability: featuregate.StabilityGenerallyAvailable,
		uiPrefix:     "/",
		// configFormat: "alloy",
	}

	cmd := &cobra.Command{
		Use:   "repl [flags] path",
		Short: "Run Grafana Alloy REPL",
		Long: `The repl subcommand allows for diagnostics and data collection from a running Alloy instance.
`,
		Args:         cobra.NoArgs,
		Example:      "alloy repl --server.http.addr 127.0.0.1:12345",
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			return r.Run(cmd)
		},
	}

	// Server flags
	cmd.Flags().
		StringVar(&r.httpAddr, "server.http.addr", r.httpAddr, "Address to use to locate the graphQL endpoints")
	cmd.Flags().StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to discover the HTTP UI at")

	// Config flags
	// cmd.Flags().StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	// cmd.Flags().BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")
	// cmd.Flags().StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	// cmd.Flags().StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	cmd.Flags().Var(&r.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	cmd.Flags().BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")

	return cmd
}

type alloyRepl struct {
	httpAddr string
	// storagePath          string
	minStability featuregate.Stability
	uiPrefix     string
	// configFormat         string
	enableCommunityComps bool
}

func (fr *alloyRepl) Run(cmd *cobra.Command) error {
	p := prompt.New(
		NewExecutor(fr).Execute,
		NewCompleter(fr).Complete,
		prompt.OptionTitle("alloy-repl: interactive alloy diagnostics"),
		prompt.OptionPrefix("alloy >> "),
		prompt.OptionInputTextColor(prompt.Green),
	)
	p.Run()

	return nil
}

type executor struct {
	cfg *alloyRepl
}

func NewExecutor(cfg *alloyRepl) *executor {
	return &executor{cfg: cfg}
}

func (e executor) Execute(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if line == "exit" || line == "quit" {
		fmt.Println("Exiting Alloy REPL.")
		os.Exit(0)
	}
}

type completer struct {
	cfg *alloyRepl
}

func NewCompleter(cfg *alloyRepl) *completer {
	return &completer{cfg: cfg}
}

func (c *completer) Complete(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "components", Description: "components list"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}
