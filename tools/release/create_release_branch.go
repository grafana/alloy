package release

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

type createReleaseBranchFlags struct {
	tag    string
	dryRun bool
}

func createReleaseBranchCommand() *cobra.Command {
	var flags createReleaseBranchFlags

	cmd := &cobra.Command{
		Use:   "create-release-branch",
		Short: "Create a release/vX.Y branch and matching backport label from a release tag",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateReleaseBranch(cmd.Context(), flags)
		},
	}

	cmd.Flags().StringVar(&flags.tag, "tag", "", "Release tag to branch from (e.g., v1.29.0)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Dry run (do not create branch)")
	_ = cmd.MarkFlagRequired("tag")

	return cmd
}

func runCreateReleaseBranch(ctx context.Context, flags createReleaseBranchFlags) error {
	majorMinor, err := version.MajorMinor(flags.tag)
	if err != nil {
		return fmt.Errorf("parsing version from tag %q: %w", flags.tag, err)
	}
	fmt.Printf("Release tag: %s (major.minor: %s)\n", flags.tag, majorMinor)

	branchName := releaseBranchPrefix + majorMinor
	fmt.Printf("Release branch: %s\n", branchName)

	backportLabel := backportLabelPrefix + majorMinor
	fmt.Printf("Backport label: %s\n", backportLabel)

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	exists, err := client.BranchExists(ctx, branchName)
	if err != nil {
		return fmt.Errorf("checking if branch exists: %w", err)
	}
	if exists {
		fmt.Printf("Branch %s already exists, skipping creation\n", branchName)
		return nil
	}

	if flags.dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would create branch: %s\n", branchName)
		fmt.Printf("Would create label: %s\n", backportLabel)
		fmt.Printf("From tag: %s\n", flags.tag)
		return nil
	}

	tagSHA, err := client.GetRefSHA(ctx, flags.tag)
	if err != nil {
		return fmt.Errorf("getting SHA for tag %s: %w", flags.tag, err)
	}
	fmt.Printf("Tag SHA: %s\n", tagSHA)

	err = client.CreateBranch(ctx, gh.CreateBranchParams{
		Branch: branchName,
		SHA:    tagSHA,
	})
	if err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}

	fmt.Printf("✅ Created branch: %s\n", branchName)
	fmt.Printf("🔗 https://github.com/%s/%s/tree/%s\n", client.Owner(), client.Repo(), branchName)

	created, err := client.EnsureLabel(ctx, gh.CreateLabelParams{
		Name:        backportLabel,
		Color:       gh.BackportLabelColor,
		Description: fmt.Sprintf("Backport to %s", branchName),
	})
	if err != nil {
		return fmt.Errorf("ensuring label: %w", err)
	}
	if created {
		fmt.Printf("✅ Created label: %s\n", backportLabel)
	} else {
		fmt.Printf("Label %s already exists\n", backportLabel)
	}

	return nil
}
