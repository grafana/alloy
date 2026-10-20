package alloycli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/internal/alloycli/clitools"
)

func toolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Utilities for various components",
		Long:  `The tools command contains a collection of utilities for components.`,
	}

	for _, t := range clitools.All() {
		cmd.AddCommand(getTools(t.Name, t.Install))
	}

	return cmd
}

func getTools(name string, installFunc func(*cobra.Command)) *cobra.Command {
	groupCommand := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Tools for the %s component", name),
	}
	installFunc(groupCommand)
	return groupCommand
}
