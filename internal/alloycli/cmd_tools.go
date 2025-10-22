package alloycli

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/spf13/cobra"
)

func toolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Utilities for various components",
		Long:  `The tools command contains a collection of utilities for components.`,
	}

	cmd.AddCommand(
		getTools("prometheus.remote_write", remotewrite.InstallTools),
	)

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
