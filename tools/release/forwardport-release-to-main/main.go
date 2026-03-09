package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/grafana/alloy/tools/release/internal/git"
	gh "github.com/grafana/alloy/tools/release/internal/github"
)

const (
	zizmorCheckName = "zizmor" // Code scanning check from github-advanced-security
	zizmorTimeout   = 5 * time.Minute
)

func main() {
	var (
		prNumber int
		dryRun   bool
	)
	flag.IntVar(&prNumber, "pr", 0, "Release-please PR number that was merged")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run (do not merge)")
	flag.Parse()

	if prNumber == 0 {
		log.Fatal("PR number is required (use --pr flag)")
	}

	ctx := context.Background()

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Get the original release-please PR details
	originalPR, err := client.GetPR(ctx, prNumber)
	if err != nil {
		log.Fatalf("Failed to get PR #%d: %v", prNumber, err)
	}

	if originalPR.GetMergedAt().IsZero() {
		log.Fatalf("PR #%d is not merged", prNumber)
	}

	// The base branch should be a release branch (e.g., release/v1.15)
	releaseBranch := originalPR.GetBase().GetRef()
	if !strings.HasPrefix(releaseBranch, "release/") {
		log.Fatalf("PR #%d base branch %s is not a release branch", prNumber, releaseBranch)
	}

	// Extract version from release branch (release/v1.15 -> v1.15)
	version := strings.TrimPrefix(releaseBranch, "release/")

	// Temp branch for the forwardport commit (so we can open a draft PR for zizmor)
	forwardportBranch := "forwardport/" + version

	fmt.Printf("🔀 Merging release branch to main after release-please PR #%d\n", prNumber)
	fmt.Printf("   Release branch: %s\n", releaseBranch)
	fmt.Printf("   Version: %s\n", version)
	fmt.Printf("   Forwardport branch: %s\n", forwardportBranch)

	// Check if the release branch is already fully merged into main
	alreadyMerged, err := client.IsBranchMergedInto(ctx, releaseBranch, "main")
	if err != nil {
		log.Fatalf("Failed to check if branch is merged: %v", err)
	}
	if alreadyMerged {
		fmt.Printf("ℹ️  Release branch %s is already merged into main\n", releaseBranch)
		return
	}

	if dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would merge: %s → main (via draft PR on %s, wait for zizmor, then push main)\n", releaseBranch, forwardportBranch)
		return
	}

	// Get the app identity for git commits
	appIdentity, err := client.GetAppIdentity(ctx)
	if err != nil {
		log.Fatalf("Failed to get app identity: %v", err)
	}

	// Configure git with app identity for commit authorship
	if err := git.ConfigureUser(appIdentity.Name, appIdentity.Email); err != nil {
		log.Fatalf("Failed to configure git: %v", err)
	}

	// Checkout main and create forwardport branch from it (so we build the commit on the side branch)
	fmt.Println("🔀 Checking out main...")
	if err := git.Checkout("main"); err != nil {
		log.Fatalf("Failed to checkout main: %v", err)
	}
	fmt.Printf("📌 Creating branch %s from main...\n", forwardportBranch)
	if err := git.CreateBranchFrom(forwardportBranch, "main"); err != nil {
		log.Fatalf("Failed to create branch %s: %v", forwardportBranch, err)
	}

	// Get the merge commit SHA from the release-please PR - this contains the
	// version bump and changelog updates we want to sync.
	mergeCommitSHA := originalPR.GetMergeCommitSHA()
	if mergeCommitSHA == "" {
		log.Fatal("Could not get merge commit SHA from PR")
	}
	fmt.Printf("   Release-please merge commit: %s\n", mergeCommitSHA[:7])

	// Merge the release branch using "ours" strategy (on the forwardport branch).
	// This creates a merge commit that records the release branch history (including tags)
	// but keeps main's content unchanged.
	commitMessage := fmt.Sprintf(`chore: Forwardport %s to main

Forwardports the %s branch to main after the %s release.

Triggered by release-please PR #%d: %s

This commit serves two purposes:

1. Records the release branch history (including tags) while keeping main's content unchanged
2. Syncs release-please changes (version bumps, changelog updates) with the main branch`,
		releaseBranch,
		releaseBranch,
		version,
		originalPR.GetNumber(),
		originalPR.GetTitle(),
	)

	fmt.Printf("🔀 Merging %s into %s (ours strategy)...\n", releaseBranch, forwardportBranch)
	if err := git.MergeOurs(releaseBranch, commitMessage); err != nil {
		log.Fatalf("Failed to merge %s: %v", releaseBranch, err)
	}

	// Cherry-pick the release-please changes and amend into the merge commit.
	fmt.Printf("📄 Cherry-picking release-please changes from %s...\n", mergeCommitSHA[:7])
	if err := git.CherryPick(mergeCommitSHA, false); err != nil {
		log.Fatalf("Failed to cherry-pick release-please commit: %v", err)
	}

	if err := git.AmendCommit(); err != nil {
		log.Fatalf("Failed to amend merge commit: %v", err)
	}

	// Push the forwardport branch (not main yet)
	fmt.Printf("📤 Pushing branch %s...\n", forwardportBranch)
	if err := git.Push(forwardportBranch); err != nil {
		log.Fatalf("Failed to push %s: %v", forwardportBranch, err)
	}
	defer cleanupBranch(ctx, client, forwardportBranch)

	// Open a draft PR so zizmor runs on the commit; we wait for it before pushing to main.
	draftPR, err := client.CreatePR(ctx, gh.CreatePRParams{
		Title: fmt.Sprintf("chore: Forwardport %s to main", releaseBranch),
		Head:  forwardportBranch,
		Base:  "main",
		Body:  fmt.Sprintf("Automated forwardport. Triggered by release-please PR #%d.\n\nDo not merge manually; the workflow will push to main after zizmor passes.", originalPR.GetNumber()),
		Draft: true,
	})
	if err != nil {
		cleanupBranch(ctx, client, forwardportBranch)
		log.Fatalf("Failed to create draft PR: %v", err)
	}
	fmt.Printf("📋 Created draft PR #%d: %s\n", draftPR.GetNumber(), draftPR.GetHTMLURL())

	if err := waitForZizmor(ctx, client, forwardportBranch); err != nil {
		cleanupBranch(ctx, client, forwardportBranch)
		log.Fatalf("Zizmor did not pass in time or failed: %v", err)
	}

	// Fast-forward main to the forwardport commit and push
	fmt.Println("🔀 Checking out main and merging forwardport branch...")
	if err := git.Checkout("main"); err != nil {
		cleanupBranch(ctx, client, forwardportBranch)
		log.Fatalf("Failed to checkout main: %v", err)
	}
	if err := git.MergeFFOnly(forwardportBranch); err != nil {
		cleanupBranch(ctx, client, forwardportBranch)
		log.Fatalf("Failed to merge %s into main: %v", forwardportBranch, err)
	}
	fmt.Println("📤 Pushing main...")
	if err := git.Push("main"); err != nil {
		cleanupBranch(ctx, client, forwardportBranch)
		log.Fatalf("Failed to push main: %v", err)
	}

	fmt.Printf("✅ Merged %s into main (forwardport PR #%d closed)\n", releaseBranch, draftPR.GetNumber())
}

// cleanupBranch deletes the temporary forwardport branch from the remote.
func cleanupBranch(ctx context.Context, client *gh.Client, branch string) {
	if err := client.DeleteBranch(ctx, branch); err != nil {
		log.Printf("⚠️  Failed to delete branch %s: %v", branch, err)
	} else {
		fmt.Printf("🗑️  Deleted branch %s\n", branch)
	}
}

// waitForZizmor polls until the zizmor check passes on ref or the timeout is reached.
func waitForZizmor(ctx context.Context, client *gh.Client, ref string) error {
	waitCtx, cancel := context.WithTimeout(ctx, zizmorTimeout)
	defer cancel()
	fmt.Printf("⏳ Waiting for %s check (timeout %s)...\n", zizmorCheckName, zizmorTimeout)
	if err := client.WaitForCheckRun(waitCtx, ref, zizmorCheckName); err != nil {
		return err
	}
	fmt.Printf("✅ %s check passed\n", zizmorCheckName)
	return nil
}
