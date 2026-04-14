package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"

	"github.com/grafana/alloy/tools/release/internal/git"
	gh "github.com/grafana/alloy/tools/release/internal/github"
)

// backportInfo holds all the data gathered during precondition checks that is
// needed to perform the backport.
type backportInfo struct {
	PRNumber       int
	OriginalPR     *github.PullRequest
	MergeCommitSHA string
	CommitSHA      string
	AppIdentity    gh.AppIdentity
	TargetBranch   string
	BackportBranch string
}

func main() {
	var (
		prNumber int
		label    string
		dryRun   bool
	)
	flag.IntVar(&prNumber, "pr", 0, "PR number to backport")
	flag.StringVar(&label, "label", "", "Backport label (e.g., backport/v1.15)")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run (do not create PR)")
	flag.Parse()

	if prNumber == 0 {
		log.Fatal("PR number is required (use --pr flag)")
	}
	if label == "" {
		log.Fatal("Label is required (use --label flag)")
	}

	if err := run(prNumber, label, dryRun); err != nil {
		log.Fatal(err)
	}
}

// run performs the backport operation. It is broken out into a separate function to allow deferred
// working copy cleanup.
func run(prNumber int, label string, dryRun bool) (retErr error) {
	version := strings.TrimPrefix(label, "backport/")
	if version == label {
		return fmt.Errorf("invalid backport label format: %s (expected backport/vX.Y)", label)
	}
	if !strings.HasPrefix(version, "v") {
		return fmt.Errorf("invalid version format: %s (expected vX.Y)", version)
	}

	targetBranch := fmt.Sprintf("release/%s", version)
	backportBranch := fmt.Sprintf("backport/pr-%d-to-%s", prNumber, version)

	fmt.Printf("🍒 Backporting PR #%d to %s\n", prNumber, targetBranch)

	ctx := context.Background()

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	info, err := resolveBackportInfo(ctx, client, prNumber, targetBranch, backportBranch)
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}

	if dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would create backport branch: %s\n", info.BackportBranch)
		fmt.Printf("Would cherry-pick commit: %s\n", info.CommitSHA)
		fmt.Printf("Would create PR: %s → %s\n", info.BackportBranch, info.TargetBranch)
		return nil
	}

	// Comment on the original PR with manual instructions if anything fails.
	defer func() {
		if retErr != nil {
			commentOnBackportFailure(ctx, client, info, retErr)
		}
	}()

	// --- Git operations: branch, cherry-pick, push, create PR ---

	if err := git.ConfigureUser(info.AppIdentity.Name, info.AppIdentity.Email); err != nil {
		return err
	}

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

	if err := git.CherryPick(info.CommitSHA, true); err != nil {
		return err
	}

	if err := git.Push(info.BackportBranch); err != nil {
		return err
	}
	fmt.Printf("✅ Pushed backport branch: %s\n", info.BackportBranch)

	backportPR, err := createBackportPR(ctx, client, info)
	if err != nil {
		return fmt.Errorf("creating backport PR: %w", err)
	}
	fmt.Printf("✅ Created backport PR: %s\n", backportPR.GetHTMLURL())

	return nil
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

	appIdentity, err := client.GetAppIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting app identity: %w", err)
	}

	// Check if backport was already merged by looking for the original PR
	// title in the release branch history.
	alreadyMerged, err := client.CommitExistsWithPattern(ctx, gh.FindCommitParams{
		Branch:  targetBranch,
		Pattern: originalPR.GetTitle(),
	})
	if err != nil {
		return nil, fmt.Errorf("checking for existing backport: %w", err)
	}
	if alreadyMerged {
		fmt.Printf("ℹ️  Backport already merged (found commit with title %q in %s)\n", originalPR.GetTitle(), targetBranch)
		return nil, nil
	}

	commitSHA, err := client.FindCommitWithPattern(ctx, gh.FindCommitParams{
		Branch:  "main",
		Pattern: fmt.Sprintf("(#%d)", prNumber),
	})
	if err != nil {
		return nil, fmt.Errorf("finding commit for PR #%d: %w", prNumber, err)
	}
	fmt.Printf("   Found commit: %s\n", commitSHA)
	fmt.Printf("   Backport branch: %s\n", backportBranch)

	return &backportInfo{
		PRNumber:       prNumber,
		OriginalPR:     originalPR,
		MergeCommitSHA: originalPR.GetMergeCommitSHA(),
		CommitSHA:      commitSHA,
		AppIdentity:    appIdentity,
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
	title := backportPRTitle(info.OriginalPR.GetTitle())

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

func backportPRTitle(originalTitle string) string {
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
		backportPRTitle(info.OriginalPR.GetTitle()),
	)

	if err := client.CreateIssueComment(ctx, info.PRNumber, comment); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to comment on PR #%d: %v\n", info.PRNumber, err)
	} else {
		fmt.Printf("📝 Added manual backport instructions to PR #%d\n", info.PRNumber)
	}
}
