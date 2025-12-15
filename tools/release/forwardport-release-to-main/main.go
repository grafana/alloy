package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	gh "github.com/grafana/alloy/tools/release/internal/github"
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

	fmt.Printf("🔀 Merging release branch to main after release-please PR #%d\n", prNumber)
	fmt.Printf("   Release branch: %s\n", releaseBranch)
	fmt.Printf("   Version: %s\n", version)

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
		fmt.Printf("Would merge: %s → main\n", releaseBranch)
		return
	}

	// Merge the release branch directly into main
	commitMessage := fmt.Sprintf("chore: forwardport %s to main\n\nForwardports the %s branch to main after the %s release.\n\nTriggered by release-please PR #%d: %s\n\nThis brings all release commits (changelog updates, version bumps, tags, etc.) from the release branch into main.",
		releaseBranch,
		releaseBranch,
		version,
		originalPR.GetNumber(),
		originalPR.GetTitle(),
	)

	commit, err := client.MergeBranch(ctx, gh.MergeBranchParams{
		Base:          "main",
		Head:          releaseBranch,
		CommitMessage: commitMessage,
	})
	if err != nil {
		log.Fatalf("Failed to merge %s into main: %v", releaseBranch, err)
	}

	fmt.Printf("✅ Merged %s into main\n", releaseBranch)
	fmt.Printf("   Commit: %s\n", commit.GetSHA())
}
