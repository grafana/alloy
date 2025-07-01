package alloycli

import (
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/alloycli/repl"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/graphql"
)

type alloyRepl struct {
	httpAddr string
	// storagePath          string
	minStability featuregate.Stability
	// uiPrefix     string
	// configFormat         string
	enableCommunityComps bool
}

type executor struct {
	cfg       *alloyRepl
	gqlClient *graphql.GraphQlClient
}

type completer struct {
	cfg       *alloyRepl
	gqlClient *graphql.GraphQlClient
}

func replCommand() *cobra.Command {
	r := &alloyRepl{
		httpAddr: "http://127.0.0.1:12345/graphql",
		// storagePath:    "data-alloy/",
		minStability: featuregate.StabilityGenerallyAvailable,
		// uiPrefix:     "/",
		// configFormat: "alloy",
	}

	cmd := &cobra.Command{
		Use:          "repl [flags]",
		Short:        "Run Grafana Alloy REPL",
		Long:         "The repl subcommand allows for interactive diagnostics and data collection from a running Alloy instance.",
		Args:         cobra.NoArgs,
		Example:      "alloy repl",
		SilenceUsage: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			return r.Run(cmd)
		},
	}

	// Server flags
	cmd.Flags().
		StringVar(
			&r.httpAddr,
			"server.graphql.endpoint",
			r.httpAddr,
			"Address of the GraphQL endpoint",
		)
	// cmd.Flags().StringVar(&r.uiPrefix, "server.http.ui-path-prefix", r.uiPrefix, "Prefix to discover the HTTP UI at")

	// Config flags
	// cmd.Flags().StringVar(&r.configFormat, "config.format", r.configFormat, fmt.Sprintf("The format of the source file. Supported formats: %s.", supportedFormatsList()))
	// cmd.Flags().BoolVar(&r.configBypassConversionErrors, "config.bypass-conversion-errors", r.configBypassConversionErrors, "Enable bypassing errors when converting")
	// cmd.Flags().StringVar(&r.configExtraArgs, "config.extra-args", r.configExtraArgs, "Extra arguments from the original format used by the converter. Multiple arguments can be passed by separating them with a space.")

	// Misc flags
	// cmd.Flags().StringVar(&r.storagePath, "storage.path", r.storagePath, "Base directory where components can store data")
	cmd.Flags().Var(&r.minStability, "stability.level", fmt.Sprintf("Minimum stability level of features to enable. Supported values: %s", strings.Join(featuregate.AllowedValues(), ", ")))
	// cmd.Flags().BoolVar(&r.enableCommunityComps, "feature.community-components.enabled", r.enableCommunityComps, "Enable community components.")

	return cmd
}

func (fr *alloyRepl) Run(cmd *cobra.Command) error {
	client := graphql.NewGraphQlClient(fr.httpAddr)

	p := prompt.New(
		NewExecutor(fr, client).Execute,
		NewCompleter(fr, client).Complete,
		prompt.OptionTitle("alloy-repl: interactive alloy diagnostics"),
		prompt.OptionPrefix("alloy >> "),
		prompt.OptionInputTextColor(prompt.Green),
	)
	p.Run()

	return nil
}

func NewExecutor(cfg *alloyRepl, gqlClient *graphql.GraphQlClient) *executor {
	return &executor{
		cfg:       cfg,
		gqlClient: gqlClient,
	}
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

	// Wrap the query in query {...} for convenience
	if !strings.HasPrefix(line, "query") {
		line = "query { " + line + " }"
	}

	response, err := e.gqlClient.Execute(line)
	if err != nil {
		fmt.Printf("Error executing query: %v\n", err)
		return
	}

	repl.PrintGraphQlResponse(response)
}

func NewCompleter(cfg *alloyRepl, gqlClient *graphql.GraphQlClient) *completer {
	return &completer{cfg: cfg, gqlClient: gqlClient}
}

func (c *completer) Complete(d prompt.Document) []prompt.Suggest {
	response, err := repl.IntrospectQueryFields(c.gqlClient)
	if err != nil {
		fmt.Printf("Error introspecting schema: %v\n", err)
		return []prompt.Suggest{}
	}

	fields := make([]prompt.Suggest, len(response))
	for i, field := range response {
		fields[i] = prompt.Suggest{
			Text: field.Name,
		}
	}

	return prompt.FilterHasPrefix(fields, d.GetWordBeforeCursor(), true)
}
