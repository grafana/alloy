package moduledeps

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/generate/moduledeps/internal"
	"github.com/grafana/alloy/tools/generate/moduledeps/internal/helpers"
	"github.com/grafana/alloy/tools/internal/git"
)

type flags struct {
	root           string
	dependencyYaml string
}

func Command() *cobra.Command {
	var flags flags

	cmd := &cobra.Command{
		Use:   "module-dependencies",
		Short: "Generate replace directives from dependency-replacements.yaml and inject them into go.mod and builder-config.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(flags)
		},
	}

	cmd.Flags().StringVar(&flags.dependencyYaml, "dependency-yaml", "", "Relative path to the dependency-replacements.yaml file")
	cmd.Flags().StringVar(&flags.root, "root", "", "path to root directory (default: git root)")
	_ = cmd.MarkFlagRequired("dependency-yaml")

	return cmd
}

func run(flags flags) error {
	if flags.root == "" {
		var err error
		flags.root, err = git.Root()
		if err != nil {
			return err
		}
	}

	fileHelper, err := helpers.NewFileHelper(flags.dependencyYaml, flags.root)
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
