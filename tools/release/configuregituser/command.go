// Package configuregituser configures git commit authorship for release tools
// using the GitHub App identity.
package configuregituser

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/release/internal/git"
	gh "github.com/grafana/alloy/tools/release/internal/github"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "configure-git-user",
		Short: "Configure git user.name and user.email for the GitHub App",
		Long: "Resolve the GitHub App bot identity from the environment and configure " +
			"git user.name and user.email for commits made by release automation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context())
		},
	}
}

func run(ctx context.Context) error {
	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	identity, err := client.GetAppIdentity(ctx)
	if err != nil {
		return fmt.Errorf("getting app identity: %w", err)
	}

	if err := git.ConfigureUser(identity.Name, identity.Email); err != nil {
		return err
	}

	fmt.Printf("Configured git user as %s <%s>\n", identity.Name, identity.Email)
	return nil
}
