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
	releaseBranchPrefix = "release/v"
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

	created, err := client.EnsureLabel(ctx, gh.CreateLabelParams{
		Name:        backportLabel,
		Color:       gh.BackportLabelColor,
		Description: fmt.Sprintf("Backport to %s", branchName),
	})
	if err != nil {
		log.Fatalf("Failed to ensure label: %v", err)
	}
	if created {
		fmt.Printf("✅ Created label: %s\n", backportLabel)
	} else {
		fmt.Printf("Label %s already exists\n", backportLabel)
	}
}
