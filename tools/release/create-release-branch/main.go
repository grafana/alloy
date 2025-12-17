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
		dryRun    bool
		sourceRef string
	)
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run (do not create branch)")
	flag.StringVar(&sourceRef, "source", "main", "Source ref to branch from")
	flag.Parse()

	ctx := context.Background()

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Read manifest to determine current version
	manifest, err := client.ReadManifest(ctx, sourceRef)
	if err != nil {
		log.Fatalf("Failed to read manifest: %v", err)
	}

	currentVersion, ok := manifest["."]
	if !ok {
		log.Fatal("No root version found in manifest (expected '.' key)")
	}
	fmt.Printf("Current version in manifest: %s\n", currentVersion)

	// Calculate next minor version
	nextMinor, err := version.NextMinor(currentVersion)
	if err != nil {
		log.Fatalf("Failed to calculate next minor version: %v", err)
	}
	fmt.Printf("Next minor version: %s\n", nextMinor)

	branchName := fmt.Sprintf("%s%s", releaseBranchPrefix, nextMinor)
	fmt.Printf("Release branch: %s\n", branchName)

	backportLabel := fmt.Sprintf("%s%s", backportLabelPrefix, nextMinor)
	fmt.Printf("Backport label: %s\n", backportLabel)

	// Check if branch already exists
	exists, err := client.BranchExists(ctx, branchName)
	if err != nil {
		log.Fatalf("Failed to check if branch exists: %v", err)
	}
	if exists {
		log.Fatalf("Branch %s already exists", branchName)
	}

	if dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Printf("Would create branch: %s\n", branchName)
		fmt.Printf("Would create label: %s\n", backportLabel)
		fmt.Printf("From: %s\n", sourceRef)
		return
	}

	// Get the SHA of the source ref
	sourceSHA, err := client.GetRefSHA(ctx, sourceRef)
	if err != nil {
		log.Fatalf("Failed to get SHA for %s: %v", sourceRef, err)
	}
	fmt.Printf("Source SHA: %s\n", sourceSHA)

	// Create the branch
	err = client.CreateBranch(ctx, gh.CreateBranchParams{
		Branch: branchName,
		SHA:    sourceSHA,
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
