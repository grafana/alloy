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

type backportCreationParams struct {
	OriginalPR     *github.PullRequest
	BackportBranch string
	TargetBranch   string
}

type backportFailureParams struct {
	PRNumber       int
	OriginalTitle  string
	MergeCommitSHA string
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

	// Parse version from label (backport/v1.15 -> v1.15)
	version := strings.TrimPrefix(label, "backport/")
	if version == label {
		log.Fatalf("Invalid backport label format: %s (expected backport/vX.Y)", label)
	}
	if !strings.HasPrefix(version, "v") {
		log.Fatalf("Invalid version format: %s (expected vX.Y)", version)
	}

	targetBranch := fmt.Sprintf("release/%s", version)
	backportBranch := fmt.Sprintf("backport/pr-%d-to-%s", prNumber, version)

	fmt.Printf("🍒 Backporting PR #%d to %s\n", prNumber, targetBranch)

	ctx := context.Background()

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Verify the target release branch exists
	exists, err := git.BranchExistsOnRemote(targetBranch)
	if err != nil {
		log.Fatalf("Failed to check if target branch exists: %v", err)
	}
	if !exists {
		log.Fatalf("Target branch %s does not exist", targetBranch)
	}

	// Check if backport branch already exists (means there's an open PR or work in progress)
	branchExists, err := git.BranchExistsOnRemote(backportBranch)
	if err != nil {
		log.Fatalf("Failed to check if backport branch exists: %v", err)
	}
	if branchExists {
		fmt.Printf("ℹ️  Backport branch %s already exists\n", backportBranch)
		return
	}

	// Get the original PR details
	originalPR, err := client.GetPR(ctx, prNumber)
	if err != nil {
		log.Fatalf("Failed to get original PR: %v", err)
	}

	// Get the merge commit SHA for cherry-pick instructions
	mergeCommitSHA := originalPR.GetMergeCommitSHA()

	// Get the app identity for git commits
	appIdentity, err := client.GetAppIdentity(ctx)
	if err != nil {
		log.Fatalf("Failed to get app identity: %v", err)
	}

	// Check if backport was already merged by looking for the original PR title in the release branch history
	alreadyMerged, err := client.CommitExistsWithPattern(ctx, gh.FindCommitParams{
		Branch:  targetBranch,
		Pattern: originalPR.GetTitle(),
	})
	if err != nil {
		log.Fatalf("Failed to check for existing backport commit: %v", err)
	}
	if alreadyMerged {
		fmt.Printf("ℹ️  Backport already merged (found commit with title %q in %s)\n", originalPR.GetTitle(), targetBranch)
		return
	}

	// Find the commit on main that corresponds to this PR
	commitSHA, err := client.FindCommitWithPattern(ctx, gh.FindCommitParams{
		Branch:  "main",
		Pattern: fmt.Sprintf("(#%d)", prNumber),
	})
	if err != nil {
		log.Fatalf("Failed to find commit for PR #%d: %v", prNumber, err)
	}
	fmt.Printf("   Found commit: %s\n", commitSHA)
	fmt.Printf("   Backport branch: %s\n", backportBranch)

	if dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would create backport branch: %s\n", backportBranch)
		fmt.Printf("Would cherry-pick commit: %s\n", commitSHA)
		fmt.Printf("Would create PR: %s → %s\n", backportBranch, targetBranch)
		return
	}

	// Configure git with app identity for commit authorship
	if err := git.ConfigureUser(appIdentity.Name, appIdentity.Email); err != nil {
		log.Fatalf("Failed to configure git: %v", err)
	}

	// Fetch target branch for cherry-pick
	if err := git.Fetch(targetBranch); err != nil {
		log.Fatalf("Failed to fetch target branch: %v", err)
	}

	// Create backport branch from target branch
	if err := git.CreateBranchFrom(backportBranch, "origin/"+targetBranch); err != nil {
		log.Fatalf("Failed to create backport branch: %v", err)
	}

	// Cherry-pick the commit
	if err := git.CherryPick(commitSHA, true); err != nil {
		commentOnBackportFailure(ctx, client, backportFailureParams{
			PRNumber:       prNumber,
			OriginalTitle:  originalPR.GetTitle(),
			MergeCommitSHA: mergeCommitSHA,
			TargetBranch:   targetBranch,
			BackportBranch: backportBranch,
		})
		fmt.Fprintf(os.Stderr, "Failed to cherry-pick commit: %v. A comment has been added to the original PR (#%d) with instructions for manual backport.\n", err, prNumber)
		os.Exit(1)
	}

	// Push the backport branch
	if err := git.Push(backportBranch); err != nil {
		log.Fatalf("Failed to push backport branch: %v", err)
	}

	fmt.Printf("✅ Pushed backport branch: %s\n", backportBranch)

	// Create the backport PR
	backportPR, err := createBackportPR(ctx, client, backportCreationParams{
		OriginalPR:     originalPR,
		BackportBranch: backportBranch,
		TargetBranch:   targetBranch,
	})
	if err != nil {
		log.Fatalf("Failed to create backport PR: %v", err)
	}

	fmt.Printf("✅ Created backport PR: %s\n", backportPR.GetHTMLURL())
}

func createBackportPR(ctx context.Context, client *gh.Client, params backportCreationParams) (*github.PullRequest, error) {
	// Use the original PR's title with [backport] suffix for conventional commit compatibility
	title := backportPRTitle(params.OriginalPR.GetTitle())

	body := fmt.Sprintf(`## Backport of #%d

This PR backports #%d to %s.

### Original PR Author
@%s

### Description
%s

---
*This backport was created automatically.*
`,
		params.OriginalPR.GetNumber(),
		params.OriginalPR.GetNumber(),
		params.TargetBranch,
		params.OriginalPR.GetUser().GetLogin(),
		params.OriginalPR.GetBody(),
	)

	return client.CreatePR(ctx, gh.CreatePRParams{
		Title: title,
		Head:  params.BackportBranch,
		Base:  params.TargetBranch,
		Body:  body,
	})
}

func backportPRTitle(originalTitle string) string {
	return fmt.Sprintf("%s [backport]", originalTitle)
}

func commentOnBackportFailure(ctx context.Context, client *gh.Client, params backportFailureParams) {
	comment := fmt.Sprintf(`## ⚠️ Automatic backport to %s failed

The automatic backport for this PR failed, likely due to merge conflicts. Perform the backport manually:

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
		params.TargetBranch,
		params.TargetBranch,
		params.TargetBranch,
		params.BackportBranch,
		params.MergeCommitSHA,
		params.BackportBranch,
		params.BackportBranch,
		params.TargetBranch,
		backportPRTitle(params.OriginalTitle),
	)

	if err := client.CreateIssueComment(ctx, params.PRNumber, comment); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to comment on PR #%d: %v\n", params.PRNumber, err)
	} else {
		fmt.Printf("📝 Added manual backport instructions to PR #%d\n", params.PRNumber)
	}
}
