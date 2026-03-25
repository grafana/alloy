package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

const (
	// backportLabelColor is the hex color for backport labels (without '#' prefix).
	backportLabelColor = "63a504"
	// releaseBranchPrefix is the prefix for release branches.
	releaseBranchPrefix = "release/v"
	// backportLabelPrefix is the prefix for backport labels.
	backportLabelPrefix = "backport/v"
)

func main() {
	var (
		dryRun bool
		tag    string
	)
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run (do not create branch)")
	flag.StringVar(&tag, "tag", "", "Release tag to branch from (e.g., v1.29.0)")
	flag.Parse()

	if tag == "" {
		log.Fatal("Release tag is required (use --tag flag, e.g., --tag v1.29.0)")
	}

	majorMinor, err := version.MajorMinor(tag)
	if err != nil {
		log.Fatalf("Failed to parse version from tag %q: %v", tag, err)
	}
	fmt.Printf("Release tag: %s (major.minor: %s)\n", tag, majorMinor)

	branchName := fmt.Sprintf("%s%s", releaseBranchPrefix, majorMinor)
	fmt.Printf("Release branch: %s\n", branchName)

	backportLabel := fmt.Sprintf("%s%s", backportLabelPrefix, majorMinor)
	fmt.Printf("Backport label: %s\n", backportLabel)

	ctx := context.Background()

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}

	exists, err := client.BranchExists(ctx, branchName)
	if err != nil {
		log.Fatalf("Failed to check if branch exists: %v", err)
	}
	if exists {
		fmt.Printf("Branch %s already exists, skipping creation\n", branchName)
		return
	}

	if dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would create branch: %s\n", branchName)
		fmt.Printf("Would create label: %s\n", backportLabel)
		fmt.Printf("From tag: %s\n", tag)
		return
	}

	// Get the SHA of the tag
	tagSHA, err := client.GetRefSHA(ctx, tag)
	if err != nil {
		log.Fatalf("Failed to get SHA for tag %s: %v", tag, err)
	}
	fmt.Printf("Tag SHA: %s\n", tagSHA)

	// Create the release branch
	err = client.CreateBranch(ctx, gh.CreateBranchParams{
		Branch: branchName,
		SHA:    tagSHA,
	})
	if err != nil {
		log.Fatalf("Failed to create branch: %v", err)
	}

	fmt.Printf("✅ Created branch: %s\n", branchName)
	fmt.Printf("🔗 https://github.com/%s/%s/tree/%s\n", client.Owner(), client.Repo(), branchName)

	// Create the backport label
	err = client.CreateLabel(ctx, gh.CreateLabelParams{
		Name:        backportLabel,
		Color:       backportLabelColor,
		Description: fmt.Sprintf("Backport to %s", branchName),
	})
	if err != nil {
		log.Fatalf("Failed to create label: %v", err)
	}

	fmt.Printf("✅ Created label: %s\n", backportLabel)
}
