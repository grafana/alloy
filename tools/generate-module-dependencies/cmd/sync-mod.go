package cmd

import (
	"log"
	"os"

	"github.com/grafana/replace-generator/internal"
	"github.com/grafana/replace-generator/internal/helpers"
	"github.com/spf13/cobra"
)

var generateAndApplyReplaces = &cobra.Command{
	Use:   "generate",
	Short: "Generates replace directives as specified in the input dependency-replacements.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		pathToYaml := cmd.Flag("dependency-yaml").Value.String()
		pathToRoot := cmd.Flag("project-root").Value.String()

		fileHelper, err := helpers.NewFileHelper(pathToYaml, pathToRoot)
		if err != nil {
			log.Fatalf("Failed to create file helper: %v", err)
		}

		projectReplaces, err := fileHelper.LoadProjectReplaces()
		if err != nil {
			log.Fatalf("Failed to load project replaces: %v", err)
		}

		modByReplaceStr := internal.GenerateReplaces(fileHelper, projectReplaces)
		internal.ApplyReplaces(fileHelper, projectReplaces, modByReplaceStr)
		internal.TidyModules(fileHelper, projectReplaces)
	},
}

func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:               "sync-mod",
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	rootCmd.AddCommand(generateAndApplyReplaces)
	generateAndApplyReplaces.Flags().String("dependency-yaml", "", "Relative path to the dependency-replacements.yaml file")
	generateAndApplyReplaces.Flags().String("project-root", "", "Relative path to the project root")

	return rootCmd
}
