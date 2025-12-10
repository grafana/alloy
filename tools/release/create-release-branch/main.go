package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

const (
	// backportLabelColor is the hex color for backport labels (without '#' prefix).
	backportLabelColor = "63a504"
	// backportLabelPrefix is the prefix for backport labels.
	backportLabelPrefix = "backport/v"
	// maxBackportLabels is the maximum number of backport labels to keep.
	maxBackportLabels = 3
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

	branchName := fmt.Sprintf("release/v%s", nextMinor)
	fmt.Printf("Release branch: %s\n", branchName)

	backportLabel := fmt.Sprintf("backport/v%s", nextMinor)
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

	// Clean up old backport labels (keep only the most recent maxBackportLabels)
	if err := cleanupOldBackportLabels(ctx, client); err != nil {
		log.Fatalf("Failed to clean up old backport labels: %v", err)
	}
}

// cleanupOldBackportLabels removes backport labels older than n-2.
func cleanupOldBackportLabels(ctx context.Context, client *gh.Client) error {
	labels, err := client.ListLabelsWithPrefix(ctx, backportLabelPrefix)
	if err != nil {
		return err
	}

	if len(labels) <= maxBackportLabels {
		return nil
	}

	// Sort labels by version (descending)
	sort.Slice(labels, func(i, j int) bool {
		majI, minI, err := parseBackportVersion(labels[i])
		if err != nil {
			return false
		}
		majJ, minJ, err := parseBackportVersion(labels[j])
		if err != nil {
			return false
		}

		if majI != majJ {
			return majI > majJ
		}
		return minI > minJ
	})

	// Delete labels beyond the max count
	for _, label := range labels[maxBackportLabels:] {
		if err := client.DeleteLabel(ctx, label); err != nil {
			return err
		}
		fmt.Printf("🗑️  Deleted old label: %s\n", label)
	}

	return nil
}

// parseBackportVersion extracts the major and minor version numbers from a backport label.
// e.g., "backport/v1.9" -> (1, 9), "backport/v2.10" -> (2, 10)
func parseBackportVersion(label string) (major int, minor int, err error) {
	// Remove "backport/" prefix to get "vX.Y", append ".0" to make valid semver
	v := strings.TrimPrefix(label, "backport/") + ".0"

	// Use version package to extract major.minor string via semver
	mm, err := version.MajorMinor(v)
	if err != nil {
		return 0, 0, err
	}

	if _, err := fmt.Sscanf(mm, "%d.%d", &major, &minor); err != nil {
		return 0, 0, fmt.Errorf("parsing major.minor from %s: %w", mm, err)
	}
	return major, minor, nil
}
