package commitworktree

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/release/internal/git"
	gh "github.com/grafana/alloy/tools/release/internal/github"
)

type flags struct {
	branch  string
	message string
}

func Command() *cobra.Command {
	var flags flags

	cmd := &cobra.Command{
		Use:   "commit-worktree",
		Short: "Commit staged changes to a branch through the GitHub API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.Flags().StringVar(&flags.branch, "branch", "", "Branch to commit to (defaults to current branch)")
	cmd.Flags().StringVar(&flags.message, "message", "", "Commit message")
	_ = cmd.MarkFlagRequired("message")

	return cmd
}

func run(ctx context.Context, flags flags) error {
	branch := flags.branch
	if branch == "" {
		var err error
		branch, err = git.CurrentBranch()
		if err != nil {
			return err
		}
	}

	changes, err := git.GetStagedChanges()
	if err != nil {
		return err
	}
	if len(changes.Additions) == 0 && len(changes.Deletions) == 0 {
		fmt.Println("No staged changes to commit.")
		return nil
	}

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	expectedHeadOID, err := client.GetRefSHA(ctx, branch)
	if err != nil {
		return fmt.Errorf("getting expected head for %s: %w", branch, err)
	}

	oid, err := createCommit(ctx, client, branch, expectedHeadOID, flags.message, changes)
	if err != nil {
		return err
	}

	fmt.Printf("Created GitHub API commit %s on %s\n", oid, branch)
	return nil
}

func createCommit(ctx context.Context, client *gh.Client, branch, expectedHeadOID, message string, changes git.StagedDiff) (string, error) {
	additions := make([]gh.FileAddition, 0, len(changes.Additions))
	for _, addition := range changes.Additions {
		additions = append(additions, gh.FileAddition{
			Path:     addition.Path,
			Contents: addition.Contents,
		})
	}

	headline, body := git.SplitCommitMessage(message)
	return client.CreateCommitOnBranch(ctx, gh.CreateCommitOnBranchParams{
		Branch:          branch,
		ExpectedHeadOID: expectedHeadOID,
		Headline:        headline,
		Body:            body,
		Additions:       additions,
		Deletions:       changes.Deletions,
	})
}
