package backport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		Use:   "prepare-backport",
		Short: "Prepare a squash-merged PR cherry-pick for a backport PR",
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
		return setGitHubOutput("skipped", "true")
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

	commitMessage, err := prepareBackportWorkspace(info)
	if err != nil {
		return err
	}

	if err := writeBackportOutputs(info, commitMessage); err != nil {
		return err
	}
	fmt.Printf("✅ Prepared backport branch %s for pull request creation\n", info.BackportBranch)

	return nil
}

func prepareBackportWorkspace(info *backportInfo) (string, error) {
	if err := git.Fetch(info.TargetBranch); err != nil {
		return "", err
	}

	if err := git.CheckoutNewBranch(info.TargetBranch, "origin/"+info.TargetBranch); err != nil {
		return "", err
	}

	if err := git.CherryPick(info.MergeCommitSHA, true); err != nil {
		return "", err
	}

	message, err := git.GetHeadCommitMessage()
	if err != nil {
		return "", err
	}
	author, err := git.GetHeadCommitAuthor()
	if err != nil {
		return "", err
	}
	message = appendOriginalAuthorTrailer(message, author)

	if err := git.ResetLastCommit(); err != nil {
		return "", err
	}

	return message, nil
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

func writeBackportOutputs(info *backportInfo, commitMessage string) error {
	bodyPath, err := writeBackportBody(info)
	if err != nil {
		return err
	}

	outputs := map[string]string{
		"skipped":        "false",
		"branch":         info.BackportBranch,
		"base":           info.TargetBranch,
		"title":          getBackportPRTitle(info.OriginalPR.GetTitle()),
		"body_path":      bodyPath,
		"commit_message": commitMessage,
	}

	for name, value := range outputs {
		if err := setGitHubOutput(name, value); err != nil {
			return err
		}
	}
	return nil
}

func writeBackportBody(info *backportInfo) (string, error) {
	dir := os.Getenv("RUNNER_TEMP")
	if dir == "" {
		dir = os.TempDir()
	}

	safeBranch := strings.NewReplacer("/", "-").Replace(info.BackportBranch)
	bodyPath := filepath.Join(dir, fmt.Sprintf("%s.md", safeBranch))
	if err := os.WriteFile(bodyPath, []byte(getBackportPRBody(info)), 0o600); err != nil {
		return "", fmt.Errorf("writing backport PR body: %w", err)
	}
	return bodyPath, nil
}

func getBackportPRBody(info *backportInfo) string {
	return fmt.Sprintf(`## Backport of #%d

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
}

func getBackportPRTitle(originalTitle string) string {
	return fmt.Sprintf("%s [backport]", originalTitle)
}

func appendOriginalAuthorTrailer(message string, author git.CommitAuthor) string {
	if author.Name == "" || author.Email == "" {
		return message
	}

	trailer := fmt.Sprintf("Co-authored-by: %s <%s>", author.Name, author.Email)
	if strings.Contains(message, trailer) {
		return message
	}

	return strings.TrimRight(message, "\n") + "\n\n" + trailer
}

func setGitHubOutput(name, value string) error {
	outputPath := os.Getenv("GITHUB_OUTPUT")
	if outputPath == "" {
		return nil
	}

	outputFile, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opening GITHUB_OUTPUT: %w", err)
	}
	defer outputFile.Close()

	delimiter := "EOF"
	for strings.Contains(value, delimiter) {
		delimiter += "_EOF"
	}

	if _, err := fmt.Fprintf(outputFile, "%s<<%s\n%s\n%s\n", name, delimiter, value, delimiter); err != nil {
		return fmt.Errorf("writing GITHUB_OUTPUT %s: %w", name, err)
	}
	return nil
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
