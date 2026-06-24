package backport

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/release/internal/git"
	gh "github.com/grafana/alloy/tools/release/internal/github"
)

type flags struct {
	prNumber int
	label    string
	dryRun   bool
}

// backportInfo holds all the data gathered during precondition checks that is
// needed to perform the backport.
type backportInfo struct {
	PRNumber       int
	OriginalPR     *github.PullRequest
	MergeCommitSHA string
	TargetBranch   string
	BackportBranch string
}

func Command() *cobra.Command {
	var flags flags

	cmd := &cobra.Command{
		Use:   "backport",
		Short: "Cherry-pick a squash-merged PR to a release branch and open a backport PR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.Flags().IntVar(&flags.prNumber, "pr", 0, "PR number to backport")
	cmd.Flags().StringVar(&flags.label, "label", "", "Backport label (e.g., backport/v1.15)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Dry run (do not create PR)")
	_ = cmd.MarkFlagRequired("pr")
	_ = cmd.MarkFlagRequired("label")

	return cmd
}

func run(ctx context.Context, flags flags) (retErr error) {
	version := strings.TrimPrefix(flags.label, "backport/")
	if version == flags.label {
		return fmt.Errorf("invalid backport label format: %s (expected backport/vX.Y)", flags.label)
	}
	if !strings.HasPrefix(version, "v") {
		return fmt.Errorf("invalid version format: %s (expected vX.Y)", version)
	}

	targetBranch := fmt.Sprintf("release/%s", version)
	backportBranch := fmt.Sprintf("backport/pr-%d-to-%s", flags.prNumber, version)

	fmt.Printf("🍒 Backporting PR #%d to %s\n", flags.prNumber, targetBranch)

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	info, err := resolveBackportInfo(ctx, client, flags.prNumber, targetBranch, backportBranch)
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}

	if flags.dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would create backport branch: %s\n", info.BackportBranch)
		fmt.Printf("Would cherry-pick commit: %s\n", info.MergeCommitSHA)
		fmt.Printf("Would create PR: %s → %s\n", info.BackportBranch, info.TargetBranch)
		return nil
	}

	// Comment on the original PR with manual instructions if anything fails.
	defer func() {
		if retErr != nil {
			commentOnBackportFailure(ctx, client, info, retErr)
		}
	}()

	// --- Git operations: branch, cherry-pick, create signed API commit, create PR ---

	if err := performBackport(ctx, client, info); err != nil {
		return err
	}

	backportPR, err := createBackportPR(ctx, client, info)
	if err != nil {
		return fmt.Errorf("creating backport PR: %w", err)
	}
	fmt.Printf("✅ Created backport PR: %s\n", backportPR.GetHTMLURL())

	return nil
}

func performBackport(ctx context.Context, client *gh.Client, info *backportInfo) error {
	if err := git.Fetch(info.TargetBranch); err != nil {
		return err
	}

	originalBranch, err := git.CurrentBranch()
	if err != nil {
		return err
	}

	if err := git.CheckoutNewBranch(info.BackportBranch, "origin/"+info.TargetBranch); err != nil {
		return err
	}
	defer resetWorkingCopy(originalBranch, info.BackportBranch)

	changes, baseSHA, err := prepareBackportCommit(ctx, client, info)
	if err != nil {
		return err
	}
	if err := client.CreateBranch(ctx, gh.CreateBranchParams{
		Branch: info.BackportBranch,
		SHA:    baseSHA,
	}); err != nil {
		return fmt.Errorf("creating backport branch: %w", err)
	}

	message, err := git.GetCherryPickCommitMessage(info.MergeCommitSHA)
	if err != nil {
		return err
	}
	commitSHA, err := createBackportCommit(ctx, client, info.BackportBranch, baseSHA, message, changes)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Created signed backport commit %s on %s\n", commitSHA, info.BackportBranch)

	return nil
}

func prepareBackportCommit(ctx context.Context, client *gh.Client, info *backportInfo) (git.StagedDiff, string, error) {
	if err := git.CherryPick(info.MergeCommitSHA, false); err != nil {
		return git.StagedDiff{}, "", err
	}

	changes, err := git.GetStagedChanges()
	if err != nil {
		return git.StagedDiff{}, "", err
	}
	if len(changes.Additions) == 0 && len(changes.Deletions) == 0 {
		return git.StagedDiff{}, "", fmt.Errorf("cherry-pick of %s produced no staged changes", info.MergeCommitSHA)
	}

	baseSHA, err := client.GetRefSHA(ctx, info.TargetBranch)
	if err != nil {
		return git.StagedDiff{}, "", fmt.Errorf("getting target branch SHA: %w", err)
	}

	return changes, baseSHA, nil
}

// resolveBackportInfo gathers all the information needed to perform a
// backport. It returns nil (with no error) when the backport can be skipped
// (target branch missing, already backported, etc.).
func resolveBackportInfo(ctx context.Context, client *gh.Client, prNumber int, targetBranch, backportBranch string) (*backportInfo, error) {
	// Verify the target release branch exists. If it doesn't, the release
	// hasn't been finalized yet (we're still in RC mode) and the change will
	// be included in the release implicitly since it's already on main.
	exists, err := git.BranchExistsOnRemote(targetBranch)
	if err != nil {
		return nil, fmt.Errorf("checking target branch: %w", err)
	}
	if !exists {
		fmt.Printf("ℹ️  Target branch %s does not exist yet (release is likely still in RC phase).\n", targetBranch)
		fmt.Printf("   No backport needed — this change will be included in the release implicitly.\n")
		return nil, nil
	}

	// Check if backport branch already exists (means there's an open PR or work in progress)
	branchExists, err := git.BranchExistsOnRemote(backportBranch)
	if err != nil {
		return nil, fmt.Errorf("checking backport branch: %w", err)
	}
	if branchExists {
		fmt.Printf("ℹ️  Backport branch %s already exists\n", backportBranch)
		return nil, nil
	}

	originalPR, err := client.GetPR(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("getting PR #%d: %w", prNumber, err)
	}

	mergeCommitSHA := originalPR.GetMergeCommitSHA()
	if mergeCommitSHA == "" {
		return nil, fmt.Errorf("PR #%d does not have a merge commit SHA", prNumber)
	}

	cherryPickedCommit, err := client.FindCherryPickedCommit(ctx, gh.FindCherryPickedCommitParams{
		Branch:      targetBranch,
		OriginalSHA: mergeCommitSHA,
	})
	if err != nil {
		return nil, fmt.Errorf("checking for existing backport: %w", err)
	}
	if cherryPickedCommit != nil {
		fmt.Printf("ℹ️  Backport already merged (found cherry-pick of %s in %s)\n", mergeCommitSHA, targetBranch)
		return nil, nil
	}

	fmt.Printf("   Found commit: %s\n", mergeCommitSHA)
	fmt.Printf("   Backport branch: %s\n", backportBranch)

	return &backportInfo{
		PRNumber:       prNumber,
		OriginalPR:     originalPR,
		MergeCommitSHA: mergeCommitSHA,
		TargetBranch:   targetBranch,
		BackportBranch: backportBranch,
	}, nil
}

// resetWorkingCopy restores the repository to a clean state so subsequent
// backports in the same run can proceed.
func resetWorkingCopy(originalBranch, backportBranch string) {
	fmt.Println("🧹 Resetting working copy for next backport...")
	_ = git.AbortCherryPick()
	_ = git.ResetHard()
	if err := git.Checkout(originalBranch); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to checkout %s: %v\n", originalBranch, err)
	}
	if err := git.DeleteLocalBranch(backportBranch); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete local branch %s: %v\n", backportBranch, err)
	}
}

func createBackportPR(ctx context.Context, client *gh.Client, info *backportInfo) (*github.PullRequest, error) {
	title := getBackportPRTitle(info.OriginalPR.GetTitle())

	body := fmt.Sprintf(`## Backport of #%d

This PR backports #%d to %s.

### Original PR Author
@%s

### Description
%s

---
*This backport was created automatically.*
`,
		info.OriginalPR.GetNumber(),
		info.OriginalPR.GetNumber(),
		info.TargetBranch,
		info.OriginalPR.GetUser().GetLogin(),
		info.OriginalPR.GetBody(),
	)

	return client.CreatePR(ctx, gh.CreatePRParams{
		Title: title,
		Head:  info.BackportBranch,
		Base:  info.TargetBranch,
		Body:  body,
	})
}

func createBackportCommit(ctx context.Context, client *gh.Client, branch, expectedHeadOID, message string, changes git.StagedDiff) (string, error) {
	additions := make([]gh.FileAddition, 0, len(changes.Additions))
	for _, addition := range changes.Additions {
		additions = append(additions, gh.FileAddition{
			Path:     addition.Path,
			Contents: addition.Contents,
		})
	}

	headline, body, _ := strings.Cut(strings.TrimSpace(message), "\n")
	return client.CreateCommitOnBranch(ctx, gh.CreateCommitOnBranchParams{
		Branch:          branch,
		ExpectedHeadOID: expectedHeadOID,
		Headline:        strings.TrimSpace(headline),
		Body:            strings.TrimSpace(body),
		Additions:       additions,
		Deletions:       changes.Deletions,
	})
}

func getBackportPRTitle(originalTitle string) string {
	return fmt.Sprintf("%s [backport]", originalTitle)
}

func commentOnBackportFailure(ctx context.Context, client *gh.Client, info *backportInfo, backportErr error) {
	comment := fmt.Sprintf(`## ⚠️ Automatic backport to %s failed

The automatic backport for this PR failed:

> %s

To perform the backport manually:

`+"```"+`bash
git fetch origin main %s
git checkout %s
git pull
git checkout -b %s
git cherry-pick -x %s
# If conflicts exist, fix them, then:
git add .
git commit
# If no [more] conflicts, then:
git push -u origin %s
`+"```"+`

Then create a PR from `+"`%s`"+` to `+"`%s`"+` with the title:

%s
`,
		info.TargetBranch,
		backportErr,
		info.TargetBranch,
		info.TargetBranch,
		info.BackportBranch,
		info.MergeCommitSHA,
		info.BackportBranch,
		info.BackportBranch,
		info.TargetBranch,
		getBackportPRTitle(info.OriginalPR.GetTitle()),
	)

	if err := client.CreateIssueComment(ctx, info.PRNumber, comment); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to comment on PR #%d: %v\n", info.PRNumber, err)
	} else {
		fmt.Printf("📝 Added manual backport instructions to PR #%d\n", info.PRNumber)
	}
}
