package generate

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/generate/internal"
	"github.com/grafana/alloy/tools/generate/internal/helpers"
)

type moduleDependenciesFlags struct {
	dependencyYaml string
	projectRoot    string
}

func moduleDependenciesCommand() *cobra.Command {
	var flags moduleDependenciesFlags

	cmd := &cobra.Command{
		Use:   "module-dependencies",
		Short: "Generate replace directives from dependency-replacements.yaml and inject them into go.mod and builder-config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleDependencies(flags)
		},
	}

	cmd.Flags().StringVar(&flags.dependencyYaml, "dependency-yaml", "", "Relative path to the dependency-replacements.yaml file")
	cmd.Flags().StringVar(&flags.projectRoot, "project-root", "", "Relative path to the project root")
	_ = cmd.MarkFlagRequired("dependency-yaml")
	_ = cmd.MarkFlagRequired("project-root")

	return cmd
}

func runModuleDependencies(flags moduleDependenciesFlags) error {
	fileHelper, err := helpers.NewFileHelper(flags.dependencyYaml, flags.projectRoot)
	if err != nil {
		return fmt.Errorf("creating file helper: %w", err)
	}

	projectReplaces, err := fileHelper.LoadProjectReplaces()
	if err != nil {
		return fmt.Errorf("loading project replaces: %w", err)
	}

	modByReplaceStr := internal.GenerateReplaces(fileHelper, projectReplaces)
	internal.ApplyReplaces(fileHelper, projectReplaces, modByReplaceStr)
	internal.TidyModules(fileHelper, projectReplaces)
	return nil
}
